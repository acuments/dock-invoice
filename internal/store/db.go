// Package store provides SQLite-backed persistence for settings, customer
// and item masters, invoice history and invoice numbering. It uses the pure
// Go modernc.org/sqlite driver so the app needs no CGO and no system SQLite.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps a SQLite connection with the invoicer schema applied.
type DB struct {
	sql *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path and applies
// the schema. Use ":memory:" for an ephemeral in-process database (handy for
// tests).
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// modernc.org/sqlite does not support concurrent writers on the same
	// connection pool well; a single connection avoids "database is
	// locked" errors for this single-process desktop app.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("store: enable foreign keys: %w", err)
	}
	if _, err := sqlDB.Exec(schemaSQL); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("store: apply schema: %w", err)
	}
	if err := migrateItemsCurrency(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("store: migrate items: %w", err)
	}
	return &DB{sql: sqlDB}, nil
}

// migrateItemsCurrency adds the items.currency column to databases created
// before saved items could be priced in INR (CREATE TABLE IF NOT EXISTS in
// schema.sql only shapes brand-new tables, so pre-existing ones need this
// explicit ALTER TABLE). Every row that predates the column was entered
// through a "Default rate (USD)" field, so backfilling 'USD' preserves
// exactly what those rates already meant — never silently reinterprets an
// existing rate as INR.
func migrateItemsCurrency(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(items)`)
	if err != nil {
		return fmt.Errorf("inspect items columns: %w", err)
	}
	defer rows.Close()

	hasCurrency := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan items column info: %w", err)
		}
		if name == "currency" {
			hasCurrency = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasCurrency {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE items ADD COLUMN currency TEXT NOT NULL DEFAULT 'USD'`); err != nil {
		return fmt.Errorf("add items.currency column: %w", err)
	}
	return nil
}

// BackupTo writes a consistent snapshot of the database to path using SQLite's
// VACUUM INTO. The destination file is created or replaced. Safe to call while
// the app is running — handy for copying data to a USB drive or cloud folder.
func (d *DB) BackupTo(path string) error {
	if _, err := d.sql.Exec(`VACUUM INTO ?`, path); err != nil {
		return fmt.Errorf("store: backup to %s: %w", path, err)
	}
	return nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.sql.Close()
}
