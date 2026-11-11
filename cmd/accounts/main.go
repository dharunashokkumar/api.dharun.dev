// Command accounts runs the account system (PocketBase, see internal/accounts) for the
// API. It listens on localhost only: the API in front passes through the addresses
// the account pages use and never PocketBase's admin screen.
//
//	ACCOUNTS_DIR   where the accounts database lives (default ./accounts). A deploy
//	               must never touch it: this is the one state that cannot be rebuilt.
//	ACCOUNTS_ADDR  listen address (default 127.0.0.1:8091)
//	PUBLIC_ORIGIN  https://api.dharun.dev, for the links in mails
//	SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, SMTP_TLS, MAIL_FROM, MAIL_FROM_NAME
//	GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET
//	ACCOUNTS_MAX_KEYS  keys per account (default 3)
//
// The first superuser (for the admin screen, reached through an SSH tunnel only) is
// made with: accounts superuser upsert EMAIL PASSWORD.
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"

	"github.com/dharunashokkumar/api.dharun.dev/internal/accounts"
)

var version = "dev"

func main() {
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir:  env("ACCOUNTS_DIR", "accounts"),
		HideStartBanner: true,
	})
	app.RootCmd.Version = version
	accounts.Setup(app, accounts.Config{
		PublicOrigin: os.Getenv("PUBLIC_ORIGIN"),
		SMTP: accounts.SMTP{
			Host:     os.Getenv("SMTP_HOST"),
			Port:     envInt("SMTP_PORT", 587),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			TLS:      strings.EqualFold(os.Getenv("SMTP_TLS"), "true"),
			From:     os.Getenv("MAIL_FROM"),
			FromName: os.Getenv("MAIL_FROM_NAME"),
		},
		GoogleID: os.Getenv("GOOGLE_CLIENT_ID"), GoogleSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubID: os.Getenv("GITHUB_CLIENT_ID"), GitHubSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		MaxKeys: envInt("ACCOUNTS_MAX_KEYS", 3),
	})

	// With no command, serve on localhost; "accounts superuser ..." and the other
	// PocketBase commands work as usual.
	if len(os.Args) == 1 {
		address := env("ACCOUNTS_ADDR", "127.0.0.1:8091")
		if host, _, err := net.SplitHostPort(address); err != nil || !loopback(host) {
			fmt.Fprintln(os.Stderr, "ACCOUNTS_ADDR must be a localhost address")
			os.Exit(1)
		}
		if err := app.Bootstrap(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := apis.Serve(app, apis.ServeConfig{HttpAddr: address, ShowStartBanner: false, AllowedOrigins: []string{"null"}}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := app.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
