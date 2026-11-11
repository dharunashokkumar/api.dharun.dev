// Package accounts is the account system: PocketBase, used as a Go library at a pinned
// version, with the small additions India API needs. It runs as its own program
// (cmd/accounts) on localhost only; the API passes through the few addresses the
// account pages use and reads the keys from its database.
//
// What is ours here: the api_keys collection (a key is shown once and stored only as
// a SHA-256 hash), a cap on keys per account, the disabled switch, login by mailed
// code only, and the settings taken from the environment. Everything
// security-sensitive (one-time codes, tokens, mail) is PocketBase's.
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
	// PublicOrigin is where the account pages are: https://api.dharun.dev.
	PublicOrigin string
	SMTP         SMTP
	// MaxKeys is how many keys an account may hold at once. Zero means 3.
	MaxKeys int
}

// SMTP is the mail service for the login codes (Amazon SES in production, Dharun
// 2026-11-11).
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	TLS      bool
	From     string // no-reply@api.dharun.dev: SES identity with DKIM and a MAIL FROM domain
	FromName string
}

const (
	usersCollection = "users"
	keysCollection  = "api_keys"
	// OTPLength and OTPLife are the login code's digits and how long it works.
	OTPLength = 6
	OTPLife   = 10 * time.Minute
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
	// Login and signup are one thing: a six-digit code mailed to the address (Dharun,
	// 2026-11-11). No passwords, so none can leak or be guessed; a code proves the
	// address, and PocketBase marks it verified on the first one. Google and GitHub
	// logins are a later part of the plan.
	users.PasswordAuth.Enabled = false
	users.OTP.Enabled = true
	users.OTP.Length = OTPLength
	users.OTP.Duration = int64(OTPLife.Seconds())
	users.OTP.EmailTemplate = core.EmailTemplate{
		Subject: "{OTP} is your India API code",
		Body: `<p>Hello,</p>
<p>Your India API login code is:</p>
<p style="font-size:28px;font-weight:600;letter-spacing:4px">{OTP}</p>
<p>It works once, for 10 minutes. Type it on the page where you asked for it.</p>
<p>If you did not ask for a code at api.dharun.dev, ignore this mail: nothing happens without it.</p>
<p>India API</p>`,
	}
	users.MFA.Enabled = false
	users.OAuth2.Enabled = false
	users.OAuth2.Providers = nil
	// A mail on every login from a new device would double the mail for no gain: the
	// code itself already tells the owner someone is logging in.
	users.AuthAlert.Enabled = false
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
