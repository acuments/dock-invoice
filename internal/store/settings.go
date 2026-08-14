package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"dock-invoice/internal/model"
)

// GetSettings returns the single settings record, or the zero value with ok
// == false if it has never been saved.
func (d *DB) GetSettings() (model.Settings, bool, error) {
	var data string
	err := d.sql.QueryRow(`SELECT data FROM settings WHERE id = 1`).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Settings{}, false, nil
	}
	if err != nil {
		return model.Settings{}, false, fmt.Errorf("store: get settings: %w", err)
	}
	var s model.Settings
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return model.Settings{}, false, fmt.Errorf("store: unmarshal settings: %w", err)
	}
	return s, true, nil
}

// SaveSettings upserts the single settings record.
func (d *DB) SaveSettings(s model.Settings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("store: marshal settings: %w", err)
	}
	_, err = d.sql.Exec(
		`INSERT INTO settings (id, data) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data`,
		string(data),
	)
	if err != nil {
		return fmt.Errorf("store: save settings: %w", err)
	}
	return nil
}
