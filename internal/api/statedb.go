package api

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// openStateDB opens a writable SQLite file under the state directory, for what the server
// keeps between requests and across deploys (request bins, sandbox accounts). Without a
// state directory it is a private in-memory database that lasts as long as the process.
func openStateDB(stateDir, name, schema string) (*sql.DB, error) {
	var dsn string
	if stateDir == "" {
		// One shared in-memory database per name, alive while a connection holds it.
		dsn = "file:" + name + "-" + time.Now().Format("150405.000000000") + "?mode=memory&cache=shared"
	} else {
		absolute, err := filepath.Abs(filepath.Join(stateDir, name+".db"))
		if err != nil {
			return nil, err
		}
		slashed := filepath.ToSlash(absolute)
		if !strings.HasPrefix(slashed, "/") {
			slashed = "/" + slashed
		}
		dsn = "file://" + (&url.URL{Path: slashed}).EscapedPath()
	}
	dsn += sep(dsn) + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite takes one writer at a time; one connection keeps writes in order without
	// "database is locked" retries, and these tables are small.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func sep(dsn string) string {
	if strings.Contains(dsn, "?") {
		return "&"
	}
	return "?"
}

// encodeJSON is a value as JSON text for a column; decodeJSON reads one back.
func encodeJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func decodeJSON(text string, into any) { json.Unmarshal([]byte(text), into) }
