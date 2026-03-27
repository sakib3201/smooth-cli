package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: enable WAL: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);

		CREATE TABLE IF NOT EXISTS processes (
			name          TEXT PRIMARY KEY,
			config_json   TEXT NOT NULL,
			last_status   TEXT NOT NULL,
			last_pid      INTEGER,
			restart_count INTEGER DEFAULT 0,
			updated_at    TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS attention_events (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			process     TEXT NOT NULL,
			pattern     TEXT NOT NULL,
			context     TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			resolved    INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS permission_requests (
			id           TEXT PRIMARY KEY,
			process      TEXT NOT NULL,
			action       TEXT NOT NULL,
			old_value    TEXT,
			new_value    TEXT NOT NULL,
			requested_at TEXT NOT NULL,
			expires_at   TEXT NOT NULL,
			approved     INTEGER,
			decided_at   TEXT
		);

		CREATE TABLE IF NOT EXISTS settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_attention_process_resolved
			ON attention_events(process, resolved);

		CREATE INDEX IF NOT EXISTS idx_permission_pending
			ON permission_requests(approved)
			WHERE approved IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}

	var v int
	row := db.QueryRow("SELECT version FROM schema_version LIMIT 1")
	if row.Scan(&v) != nil {
		_, err = db.Exec("INSERT INTO schema_version(version) VALUES(?)", schemaVersion)
	}
	return err
}
