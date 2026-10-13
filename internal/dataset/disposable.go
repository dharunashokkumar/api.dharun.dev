package dataset

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"
)

// Domains that hand out throwaway mailboxes, from the community-kept list at
// github.com/disposable-email-domains/disposable-email-domains (CC0).

func init() {
	register(&Spec{
		Name:   "disposable-email",
		Schema: 1,
		DDL:    `CREATE TABLE domains (domain TEXT PRIMARY KEY) WITHOUT ROWID;`,
		Build:  buildDisposable,
		Import: importDisposable,
		Every:  7 * 24 * time.Hour,
		Main:   "domains", MaxShrink: 0.1,
	})
}

func buildDisposable(b *Builder) error {
	raw, err := os.ReadFile(b.Path("email", "disposable_email_blocklist.conf"))
	if err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR IGNORE INTO domains VALUES (?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		domain := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if domain == "" || strings.HasPrefix(domain, "#") {
			continue
		}
		if !strings.Contains(domain, ".") || strings.ContainsAny(domain, " /@") {
			return fmt.Errorf("%q is not a domain", domain)
		}
		if _, err := insert.Exec(domain); err != nil {
			return err
		}
		b.Add("domains", 1)
	}
	return scanner.Err()
}

const disposableList = "https://raw.githubusercontent.com/disposable-email-domains/disposable-email-domains/main/disposable_email_blocklist.conf"

func importDisposable(im *Importer) (Source, error) {
	raw, err := im.Get(disposableList)
	if err != nil {
		return Source{}, err
	}
	if bytes.Count(raw, []byte("\n")) < 1000 {
		return Source{}, fmt.Errorf("the list has only %d lines, expected thousands", bytes.Count(raw, []byte("\n")))
	}
	version := digest(raw)[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	if err := im.Write("email/disposable_email_blocklist.conf", raw); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "Disposable email domains",
		Publisher:   "github.com/disposable-email-domains/disposable-email-domains contributors",
		URL:         "https://github.com/disposable-email-domains/disposable-email-domains",
		Licence:     "CC0 1.0",
		LicenceURL:  "https://creativecommons.org/publicdomain/zero/1.0/",
		Version:     version,
		SHA256:      digest(raw),
		Attribution: "Disposable domain list from the disposable-email-domains project (CC0).",
	}, nil
}
