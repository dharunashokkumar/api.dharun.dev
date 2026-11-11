// Package accounts is the account system: PocketBase, used as a Go library at a pinned
// version, with the small additions India API needs. It runs as its own program
// (cmd/accounts) on localhost only; the API passes through the few addresses the
// account pages use and reads the keys from its database.
//
// What is ours here: the api_keys collection (a key is shown once and stored only as
// a SHA-256 hash), a cap on keys per account, the disabled switch, the mail templates
// that link to our own pages, and the settings taken from the environment. Everything
// security-sensitive (passwords, tokens, OAuth, mail) is PocketBase's.
package accounts

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/dharunashokkumar/api.dharun.dev/internal/apikey"
)

// Config is read from the environment by cmd/accounts.
type Config struct {
	// PublicOrigin is where the account pages are, for the links in mails and the OAuth
	// redirect: https://api.dharun.dev.
	PublicOrigin string
	SMTP         SMTP
	// OAuth client ids and secrets. A provider with no id is left switched off.
	GoogleID, GoogleSecret string
	GitHubID, GitHubSecret string
	// MaxKeys is how many keys an account may hold at once. Zero means 3.
	MaxKeys int
}

// SMTP is the mail service for verification and password reset mails.
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	TLS      bool
	From     string // an address at dharun.dev with SPF and DKIM set
	FromName string
}

const (
	usersCollection = "users"
	keysCollection  = "api_keys"
)

// Setup adds India API's parts to a PocketBase app. Call it before the app starts.
func Setup(app core.App, cfg Config) {
	if cfg.MaxKeys <= 0 {
		cfg.MaxKeys = 3
	}
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if err := Prepare(se.App, cfg); err != nil {
			return err
		}
		routes(se, cfg)
		return se.Next()
	})
}

// Prepare brings the collections and settings to the shape India API needs. It is
// safe to run at every start: it only changes what differs.
func Prepare(app core.App, cfg Config) error {
	if err := prepareSettings(app, cfg); err != nil {
		return err
	}
	users, err := app.FindCollectionByNameOrId(usersCollection)
	if err != nil {
		return err
	}
	if users.Fields.GetByName("disabled") == nil {
		users.Fields.Add(&core.BoolField{Name: "disabled"})
	}
	if users.Fields.GetByName("tier") == nil {
		users.Fields.Add(&core.TextField{Name: "tier", Max: 20})
	}
	// Nobody sets their own tier or switches their own account back on, and accounts are
	// made only through the API's signup address (which checks the bot challenge).
	guard := "@request.body.disabled:isset = false && @request.body.tier:isset = false && @request.body.verified:isset = false"
	owner := "id = @request.auth.id"
	users.ListRule = types.Pointer(owner)
	users.ViewRule = types.Pointer(owner)
	users.CreateRule = types.Pointer(guard)
	users.UpdateRule = types.Pointer(owner + " && " + guard)
	users.DeleteRule = types.Pointer(owner)
	// A disabled account cannot log in.
	users.AuthRule = types.Pointer("disabled = false")
	users.PasswordAuth.Enabled = true
	users.PasswordAuth.IdentityFields = []string{core.FieldNameEmail}
	users.VerificationTemplate = core.EmailTemplate{
		Subject: "Confirm your India API email",
		Body: `<p>Hello,</p>
<p>Confirm this address to start using your free India API key:</p>
<p><a class="btn" href="{APP_URL}/account/verify#{TOKEN}" target="_blank" rel="noopener">Confirm my email</a></p>
<p>If you did not sign up at api.dharun.dev, ignore this mail. Nothing happens without the click.</p>
<p>India API</p>`,
	}
	users.ResetPasswordTemplate = core.EmailTemplate{
		Subject: "Reset your India API password",
		Body: `<p>Hello,</p>
<p>Choose a new password here (the link works for 30 minutes):</p>
<p><a class="btn" href="{APP_URL}/account/reset#{TOKEN}" target="_blank" rel="noopener">Choose a new password</a></p>
<p>If you did not ask for this, ignore this mail; your password stays as it is.</p>
<p>India API</p>`,
	}
	var providers []core.OAuth2ProviderConfig
	if cfg.GoogleID != "" {
		providers = append(providers, core.OAuth2ProviderConfig{Name: "google", ClientId: cfg.GoogleID, ClientSecret: cfg.GoogleSecret})
	}
	if cfg.GitHubID != "" {
		providers = append(providers, core.OAuth2ProviderConfig{Name: "github", ClientId: cfg.GitHubID, ClientSecret: cfg.GitHubSecret})
	}
	users.OAuth2.Providers = providers
	users.OAuth2.Enabled = len(providers) > 0
	if err := app.Save(users); err != nil {
		return err
	}

	keys, err := app.FindCollectionByNameOrId(keysCollection)
	if err != nil {
		keys = core.NewBaseCollection(keysCollection)
	}
	if keys.Fields.GetByName("user") == nil {
		keys.Fields.Add(&core.RelationField{Name: "user", CollectionId: users.Id, CascadeDelete: true, Required: true, MaxSelect: 1})
		keys.Fields.Add(&core.TextField{Name: "name", Max: 60})
		keys.Fields.Add(&core.TextField{Name: "hash", Required: true, Hidden: true, Max: 64})
		keys.Fields.Add(&core.TextField{Name: "hint", Max: 40})
		keys.Fields.Add(&core.BoolField{Name: "revoked"})
		keys.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		keys.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
		keys.AddIndex("idx_api_keys_hash", true, "hash", "")
		keys.AddIndex("idx_api_keys_user", false, "user", "")
	}
	// No rules: keys are reached only through the routes below.
	keys.ListRule, keys.ViewRule, keys.CreateRule, keys.UpdateRule, keys.DeleteRule = nil, nil, nil, nil, nil
	return app.Save(keys)
}

func prepareSettings(app core.App, cfg Config) error {
	settings := app.Settings()
	settings.Meta.AppName = "India API"
	if cfg.PublicOrigin != "" {
		settings.Meta.AppURL = strings.TrimRight(cfg.PublicOrigin, "/")
	}
	if cfg.SMTP.From != "" {
		settings.Meta.SenderAddress = cfg.SMTP.From
		settings.Meta.SenderName = cfg.SMTP.FromName
		if settings.Meta.SenderName == "" {
			settings.Meta.SenderName = "India API"
		}
	}
	if cfg.SMTP.Host != "" {
		settings.SMTP.Enabled = true
		settings.SMTP.Host = cfg.SMTP.Host
		settings.SMTP.Port = cfg.SMTP.Port
		settings.SMTP.Username = cfg.SMTP.Username
		settings.SMTP.Password = cfg.SMTP.Password
		settings.SMTP.TLS = cfg.SMTP.TLS
	}
	// The API in front does the rate limiting and sees the real address; PocketBase sees
	// only localhost. Its own request log would hold addresses, so it is off: the
	// privacy page says addresses are never logged.
	settings.RateLimits.Enabled = false
	settings.Logs.MaxDays = 0
	settings.Logs.LogIP = false
	settings.Logs.LogAuthId = false
	return app.Save(settings)
}

// keyView is a key as the account page shows it: never the key itself, only its hint.
type keyView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Hint    string `json:"hint"`
	Created string `json:"created"`
	Revoked bool   `json:"revoked"`
}

func viewKey(record *core.Record) keyView {
	return keyView{
		ID: record.Id, Name: record.GetString("name"), Hint: record.GetString("hint"),
		Created: record.GetDateTime("created").Time().UTC().Format(time.RFC3339), Revoked: record.GetBool("revoked"),
	}
}

func routes(se *core.ServeEvent, cfg Config) {
	group := se.Router.Group("/api/india")
	group.Bind(apis.RequireAuth(usersCollection))

	group.GET("/me", func(e *core.RequestEvent) error {
		keys, err := userKeys(e.App, e.Auth.Id)
		if err != nil {
			return e.InternalServerError("", err)
		}
		views := make([]keyView, 0, len(keys))
		for _, key := range keys {
			views = append(views, viewKey(key))
		}
		return e.JSON(http.StatusOK, map[string]any{
			"id": e.Auth.Id, "email": e.Auth.Email(), "verified": e.Auth.Verified(),
			"created": e.Auth.GetDateTime("created").Time().UTC().Format(time.RFC3339),
			"tier":    tierOf(e.Auth), "keys": views, "maxKeys": cfg.MaxKeys,
		})
	})

	group.POST("/keys", func(e *core.RequestEvent) error {
		var body struct {
			Name string `json:"name"`
		}
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Send a JSON body such as {\"name\": \"my app\"}.", nil)
		}
		if !e.Auth.Verified() {
			return e.ForbiddenError("Confirm your email address first: we sent you a link.", nil)
		}
		active, err := e.App.CountRecords(keysCollection, dbx.HashExp{"user": e.Auth.Id, "revoked": false})
		if err != nil {
			return e.InternalServerError("", err)
		}
		if int(active) >= cfg.MaxKeys {
			return e.BadRequestError("An account holds at most "+strconv.Itoa(cfg.MaxKeys)+" keys at once. Revoke one first.", nil)
		}
		record, key, err := newKey(e.App, e.Auth.Id, body.Name)
		if err != nil {
			return e.InternalServerError("", err)
		}
		view := viewKey(record)
		return e.JSON(http.StatusOK, map[string]any{"key": key, "id": view.ID, "name": view.Name, "hint": view.Hint, "created": view.Created,
			"note": "This is the only time the key is shown. Keep it somewhere safe."})
	})

	group.POST("/keys/{id}/revoke", func(e *core.RequestEvent) error {
		record, err := ownKey(e)
		if err != nil {
			return err
		}
		record.Set("revoked", true)
		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("", err)
		}
		return e.JSON(http.StatusOK, viewKey(record))
	})

	group.POST("/keys/{id}/rotate", func(e *core.RequestEvent) error {
		record, err := ownKey(e)
		if err != nil {
			return err
		}
		if record.GetBool("revoked") {
			return e.BadRequestError("This key is already revoked.", nil)
		}
		var fresh *core.Record
		var key string
		err = e.App.RunInTransaction(func(tx core.App) error {
			record.Set("revoked", true)
			if err := tx.Save(record); err != nil {
				return err
			}
			var err error
			fresh, key, err = newKey(tx, e.Auth.Id, record.GetString("name"))
			return err
		})
		if err != nil {
			return e.InternalServerError("", err)
		}
		view := viewKey(fresh)
		return e.JSON(http.StatusOK, map[string]any{"key": key, "id": view.ID, "name": view.Name, "hint": view.Hint, "created": view.Created,
			"note": "The old key stops working within a minute. This is the only time the new key is shown."})
	})

	group.POST("/password", func(e *core.RequestEvent) error {
		var body struct {
			OldPassword string `json:"oldPassword"`
			Password    string `json:"password"`
		}
		if err := e.BindBody(&body); err != nil || body.Password == "" {
			return e.BadRequestError("Send oldPassword and password.", nil)
		}
		if len(body.Password) < 10 || len(body.Password) > 128 {
			return e.BadRequestError("A password is 10 to 128 characters.", nil)
		}
		record, err := e.App.FindRecordById(usersCollection, e.Auth.Id)
		if err != nil {
			return e.NotFoundError("", nil)
		}
		if record.GetString("password") != "" && !record.ValidatePassword(body.OldPassword) {
			return e.BadRequestError("The current password is not right.", nil)
		}
		// Setting a password also renews the token key, which logs out every session.
		record.SetPassword(body.Password)
		if err := e.App.Save(record); err != nil {
			return e.BadRequestError("That password was not accepted.", nil)
		}
		return e.JSON(http.StatusOK, map[string]any{"changed": true, "note": "Every session is logged out; log in again."})
	})

	group.POST("/logout-all", func(e *core.RequestEvent) error {
		record, err := e.App.FindRecordById(usersCollection, e.Auth.Id)
		if err != nil {
			return e.NotFoundError("", nil)
		}
		record.RefreshTokenKey()
		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("", err)
		}
		return e.JSON(http.StatusOK, map[string]any{"loggedOut": true})
	})

	group.POST("/delete-account", func(e *core.RequestEvent) error {
		var body struct {
			Confirm string `json:"confirm"`
		}
		if err := e.BindBody(&body); err != nil || !strings.EqualFold(strings.TrimSpace(body.Confirm), e.Auth.Email()) {
			return e.BadRequestError("Type your email address to confirm.", nil)
		}
		record, err := e.App.FindRecordById(usersCollection, e.Auth.Id)
		if err != nil {
			return e.NotFoundError("", nil)
		}
		// The keys go with the account (cascade); the API drops their usage counts when
		// it no longer finds them.
		if err := e.App.Delete(record); err != nil {
			return e.InternalServerError("", err)
		}
		return e.JSON(http.StatusOK, map[string]any{"deleted": true})
	})
}

func tierOf(record *core.Record) string {
	if tier := record.GetString("tier"); tier != "" {
		return tier
	}
	return "free"
}

func userKeys(app core.App, user string) ([]*core.Record, error) {
	return app.FindRecordsByFilter(keysCollection, "user = {:user}", "-created", 50, 0, dbx.Params{"user": user})
}

func ownKey(e *core.RequestEvent) (*core.Record, error) {
	record, err := e.App.FindRecordById(keysCollection, e.Request.PathValue("id"))
	if err != nil || record.GetString("user") != e.Auth.Id {
		return nil, e.NotFoundError("No such key on this account.", nil)
	}
	return record, nil
}

func newKey(app core.App, user, name string) (*core.Record, string, error) {
	collection, err := app.FindCollectionByNameOrId(keysCollection)
	if err != nil {
		return nil, "", err
	}
	key, err := apikey.New()
	if err != nil {
		return nil, "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "key"
	}
	if len(name) > 60 {
		return nil, "", errors.New("name too long")
	}
	record := core.NewRecord(collection)
	record.Set("user", user)
	record.Set("name", name)
	record.Set("hash", apikey.Hash(key))
	record.Set("hint", apikey.Hint(key))
	record.Set("revoked", false)
	if err := app.Save(record); err != nil {
		return nil, "", err
	}
	return record, key, nil
}
