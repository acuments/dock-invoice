package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"dock-invoice/internal/numbering"
)

// NextInvoiceNumber atomically allocates and returns the next invoice number
// for the given pattern (e.g. "AEX{FY}-{SEQ}") and date. Each distinct
// (pattern, financial-year) pair gets its own independent counter starting
// at 1, per numbering.SeriesKey.
func (d *DB) NextInvoiceNumber(pattern string, date time.Time) (string, error) {
	key := numbering.SeriesKey(pattern, date)

	tx, err := d.sql.Begin()
	if err != nil {
		return "", fmt.Errorf("store: begin numbering tx: %w", err)
	}
	defer tx.Rollback()

	var last int64
	err = tx.QueryRow(`SELECT last_seq FROM number_series WHERE series_key = ?`, key).Scan(&last)
	switch {
	case err == nil:
		last++
		if _, err := tx.Exec(`UPDATE number_series SET last_seq = ? WHERE series_key = ?`, last, key); err != nil {
			return "", fmt.Errorf("store: update series %q: %w", key, err)
		}
	case errors.Is(err, sql.ErrNoRows):
		// No row yet: start this series at 1.
		last = 1
		if _, err := tx.Exec(`INSERT INTO number_series (series_key, last_seq) VALUES (?, ?)`, key, last); err != nil {
			return "", fmt.Errorf("store: init series %q: %w", key, err)
		}
	default:
		return "", fmt.Errorf("store: query series %q: %w", key, err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("store: commit numbering tx: %w", err)
	}

	return numbering.Expand(pattern, date, last), nil
}
