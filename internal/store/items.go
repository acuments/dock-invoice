package store

import (
	"fmt"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

// SaveItem inserts a new saved item template and returns its ID.
func (d *DB) SaveItem(it model.Item) (int64, error) {
	res, err := d.sql.Exec(
		`INSERT INTO items (description, hsn_sac, unit, rate_usd, currency) VALUES (?, ?, ?, ?, ?)`,
		it.Description, it.HSNSAC, it.Unit, int64(it.DefaultRate), normalizeItemCurrency(it.Currency),
	)
	if err != nil {
		return 0, fmt.Errorf("store: insert item: %w", err)
	}
	return res.LastInsertId()
}

// UpdateItem overwrites an existing saved item template by ID.
func (d *DB) UpdateItem(it model.Item) error {
	_, err := d.sql.Exec(
		`UPDATE items SET description = ?, hsn_sac = ?, unit = ?, rate_usd = ?, currency = ? WHERE id = ?`,
		it.Description, it.HSNSAC, it.Unit, int64(it.DefaultRate), normalizeItemCurrency(it.Currency), it.ID,
	)
	if err != nil {
		return fmt.Errorf("store: update item %d: %w", it.ID, err)
	}
	return nil
}

// ListItems returns all saved item templates ordered by description.
func (d *DB) ListItems() ([]model.Item, error) {
	rows, err := d.sql.Query(`SELECT id, description, hsn_sac, unit, rate_usd, currency FROM items ORDER BY description`)
	if err != nil {
		return nil, fmt.Errorf("store: list items: %w", err)
	}
	defer rows.Close()

	var out []model.Item
	for rows.Next() {
		var it model.Item
		var rate int64
		var currency string
		if err := rows.Scan(&it.ID, &it.Description, &it.HSNSAC, &it.Unit, &rate, &currency); err != nil {
			return nil, fmt.Errorf("store: scan item: %w", err)
		}
		it.DefaultRate = money.Amount(rate)
		it.Currency = normalizeItemCurrency(currency)
		out = append(out, it)
	}
	return out, rows.Err()
}

// normalizeItemCurrency defaults an empty/unrecognized currency to USD —
// the assumption every saved rate made before this field existed — so a
// blank value (a freshly-migrated row, or a caller that never set it)
// never gets stored or read back as something other than what it always
// meant.
func normalizeItemCurrency(c string) string {
	if c == model.CurrencyINR {
		return model.CurrencyINR
	}
	return model.CurrencyUSD
}
