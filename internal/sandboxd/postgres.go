//go:build linux

package sandboxd

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// Throwaway Postgres: one server in the sandbox VM, a database and a user of the same
// name per request, dropped after an hour. pg_hba lets a user into its own database only
// (sameuser), and only from the API's address; the API carries callers' connections to it
// over WebSocket. Each database's account and creation time are kept as its comment, so a
// restart of the controller loses nothing.

// PostgresConfig says how to reach the server as its superuser.
type PostgresConfig struct {
	Enabled bool
	// PSQL runs psql as the superuser over the local socket, such as
	// runuser -u postgres -- psql.
	PSQL []string
}

type postgres struct {
	cfg       PostgresConfig
	metrics   *metrics
	mu        sync.Mutex
	databases map[string]sandbox.Database
	lastSweep time.Time
}

func newPostgres(cfg PostgresConfig, m *metrics) (*postgres, error) {
	p := &postgres{cfg: cfg, metrics: m, databases: map[string]sandbox.Database{}}
	if err := p.load(); err != nil {
		return nil, err
	}
	return p, nil
}

// psql runs statements, each on its own (CREATE DATABASE refuses a transaction), and
// returns the rows of the last as tab-separated lines.
func (p *postgres) psql(statements ...string) ([]string, error) {
	args := append(append([]string{}, p.cfg.PSQL[1:]...), "-X", "-q", "-t", "-A", "-F", "\t", "-v", "ON_ERROR_STOP=1", "-d", "postgres")
	for _, statement := range statements {
		args = append(args, "-c", statement)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, p.cfg.PSQL[0], args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("psql: %v: %s", err, strings.TrimSpace(string(out)))
	}
	var rows []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			rows = append(rows, line)
		}
	}
	return rows, nil
}

type databaseNote struct {
	Account string `json:"account"`
	Created int64  `json:"created"`
}

// load reads the databases that already exist.
func (p *postgres) load() error {
	rows, err := p.psql(`SELECT datname, pg_database_size(oid), coalesce(shobj_description(oid, 'pg_database'), '') FROM pg_database WHERE datname LIKE 'sb\_%'`)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, row := range rows {
		fields := strings.SplitN(row, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		var note databaseNote
		json.Unmarshal([]byte(fields[2]), &note)
		size, _ := strconv.ParseInt(fields[1], 10, 64)
		created := time.Unix(note.Created, 0)
		p.databases[fields[0]] = sandbox.Database{Name: fields[0], User: fields[0], Account: note.Account, SizeBytes: size,
			LimitBytes: sandbox.DatabaseLimit, CreatedAt: created.UTC().Format(time.RFC3339),
			ExpiresAt: created.Add(sandbox.DatabaseLifetime).UTC().Format(time.RFC3339)}
	}
	return nil
}

func (p *postgres) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.databases)
}

const passwordLetters = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func randomPassword(n int) string {
	var b strings.Builder
	for range n {
		i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(passwordLetters))))
		b.WriteByte(passwordLetters[i.Int64()])
	}
	return b.String()
}

var errTooMany = errors.New("this account already has as many databases as allowed")

func (p *postgres) create(account string) (sandbox.Database, error) {
	p.mu.Lock()
	open := 0
	for _, database := range p.databases {
		if database.Account == account {
			open++
		}
	}
	p.mu.Unlock()
	if open >= sandbox.DatabasesPerUser {
		return sandbox.Database{}, errTooMany
	}
	name := randomName("sb_")
	password := randomPassword(24)
	now := time.Now()
	note, _ := json.Marshal(databaseNote{Account: account, Created: now.Unix()})
	// Names and the password are made here from a fixed alphabet, so quoting is safe.
	_, err := p.psql(
		fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s' CONNECTION LIMIT %d NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`,
			name, password, sandbox.DatabaseConnections),
		fmt.Sprintf(`ALTER ROLE %s SET statement_timeout = '30s'`, name),
		fmt.Sprintf(`ALTER ROLE %s SET idle_in_transaction_session_timeout = '60s'`, name),
		fmt.Sprintf(`ALTER ROLE %s SET temp_file_limit = '100MB'`, name),
		fmt.Sprintf(`ALTER ROLE %s SET work_mem = '8MB'`, name),
		fmt.Sprintf(`CREATE DATABASE %s OWNER %s CONNECTION LIMIT %d TEMPLATE template0`, name, name, sandbox.DatabaseConnections),
		fmt.Sprintf(`REVOKE ALL ON DATABASE %s FROM PUBLIC`, name),
		fmt.Sprintf(`GRANT CONNECT, TEMPORARY ON DATABASE %s TO %s`, name, name),
		fmt.Sprintf(`COMMENT ON DATABASE %s IS '%s'`, name, strings.ReplaceAll(string(note), "'", "''")),
	)
	if err != nil {
		p.drop(name)
		return sandbox.Database{}, err
	}
	database := sandbox.Database{Name: name, User: name, Password: password, Account: account, LimitBytes: sandbox.DatabaseLimit,
		CreatedAt: now.UTC().Format(time.RFC3339), ExpiresAt: now.Add(sandbox.DatabaseLifetime).UTC().Format(time.RFC3339)}
	p.mu.Lock()
	stored := database
	stored.Password = ""
	p.databases[name] = stored
	p.mu.Unlock()
	p.metrics.databases.add("created", 1)
	return database, nil
}

func (p *postgres) get(name string) (sandbox.Database, error) {
	p.mu.Lock()
	database, ok := p.databases[name]
	p.mu.Unlock()
	if !ok {
		return sandbox.Database{}, errNotFound
	}
	if rows, err := p.psql(fmt.Sprintf(`SELECT pg_database_size('%s')`, name)); err == nil && len(rows) == 1 {
		database.SizeBytes, _ = strconv.ParseInt(rows[0], 10, 64)
	}
	return database, nil
}

// drop removes a database and its user, cutting off whoever is connected.
func (p *postgres) drop(name string) error {
	if !strings.HasPrefix(name, "sb_") || strings.ContainsAny(name, `"'; `) {
		return errNotFound
	}
	_, err := p.psql(
		fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, name),
		fmt.Sprintf(`DROP ROLE IF EXISTS %s`, name),
	)
	p.mu.Lock()
	delete(p.databases, name)
	p.mu.Unlock()
	return err
}

// sweep drops databases past their hour and locks those over the size limit. It runs
// every 30 seconds at most.
func (p *postgres) sweep() {
	if time.Since(p.lastSweep) < 30*time.Second {
		return
	}
	p.lastSweep = time.Now()
	rows, err := p.psql(`SELECT datname, pg_database_size(oid), datconnlimit FROM pg_database WHERE datname LIKE 'sb\_%'`)
	if err != nil {
		logf("postgres sweep: %v", err)
		return
	}
	now := time.Now()
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if len(fields) != 3 {
			continue
		}
		name := fields[0]
		size, _ := strconv.ParseInt(fields[1], 10, 64)
		p.mu.Lock()
		database, known := p.databases[name]
		if known {
			database.SizeBytes = size
			p.databases[name] = database
		}
		p.mu.Unlock()
		expires, _ := time.Parse(time.RFC3339, database.ExpiresAt)
		switch {
		case !known || now.After(expires):
			if p.drop(name) == nil {
				p.metrics.databases.add("expired", 1)
			}
		case size > sandbox.DatabaseLimit && fields[2] != "0":
			// Over the limit: nobody may connect any more; it can still be dropped.
			if _, err := p.psql(fmt.Sprintf(`ALTER DATABASE %s CONNECTION LIMIT 0`, name),
				fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s'`, name)); err == nil {
				p.metrics.databases.add("full", 1)
			}
		}
	}
}

func (p *postgres) close() {}
