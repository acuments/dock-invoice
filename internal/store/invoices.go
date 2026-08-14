package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"dock-invoice/internal/model"
)

// SaveInvoice inserts a full invoice snapshot and returns its new ID. The
// entire model.Invoice (including the already-snapshotted Customer, Seller
// and Bank fields) is stored as JSON, so later edits to the customers or
// settings tables can never retroactively change a saved invoice.
func (d *DB) SaveInvoice(inv model.Invoice) (int64, error) {
	data, err := json.Marshal(inv)
	if err != nil {
		return 0, fmt.Errorf("store: marshal invoice: %w", err)
	}
	res, err := d.sql.Exec(
		`INSERT INTO invoices (number, type, customer_name, invoice_date, data) VALUES (?, ?, ?, ?, ?)`,
		inv.Number, string(inv.Type), inv.Customer.Name, inv.Date.Format("2006-01-02"), string(data),
	)
	if err != nil {
		return 0, fmt.Errorf("store: insert invoice: %w", err)
	}
	return res.LastInsertId()
}

// UpdateInvoice overwrites an existing invoice snapshot in place, keyed by
// inv.ID. Used by the editor's "Edit" flow, as opposed to SaveInvoice which
// always inserts a new row (used for "New" and "Clone").
func (d *DB) UpdateInvoice(inv model.Invoice) error {
	data, err := json.Marshal(inv)
	if err != nil {
		return fmt.Errorf("store: marshal invoice: %w", err)
	}
	res, err := d.sql.Exec(
		`UPDATE invoices SET number = ?, type = ?, customer_name = ?, invoice_date = ?, data = ? WHERE id = ?`,
		inv.Number, string(inv.Type), inv.Customer.Name, inv.Date.Format("2006-01-02"), string(data), inv.ID,
	)
	if err != nil {
		return fmt.Errorf("store: update invoice %d: %w", inv.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: update invoice %d: %w", inv.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("store: update invoice %d: not found", inv.ID)
	}
	return nil
}

// GetInvoice fetches a full invoice snapshot by ID.
func (d *DB) GetInvoice(id int64) (model.Invoice, error) {
	var data string
	err := d.sql.QueryRow(`SELECT data FROM invoices WHERE id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Invoice{}, fmt.Errorf("store: invoice %d not found", id)
	}
	if err != nil {
		return model.Invoice{}, fmt.Errorf("store: get invoice %d: %w", id, err)
	}
	var inv model.Invoice
	if err := json.Unmarshal([]byte(data), &inv); err != nil {
		return model.Invoice{}, fmt.Errorf("store: unmarshal invoice %d: %w", id, err)
	}
	inv.ID = id
	return inv, nil
}

// invoiceRowID is a small helper struct for ListInvoices.
type invoiceRowID struct {
	id   int64
	data string
}

// ListInvoices returns every saved invoice snapshot, most recent first.
func (d *DB) ListInvoices() ([]model.Invoice, error) {
	rows, err := d.sql.Query(`SELECT id, data FROM invoices ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: list invoices: %w", err)
	}
	defer rows.Close()

	var raw []invoiceRowID
	for rows.Next() {
		var r invoiceRowID
		if err := rows.Scan(&r.id, &r.data); err != nil {
			return nil, fmt.Errorf("store: scan invoice: %w", err)
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]model.Invoice, 0, len(raw))
	for _, r := range raw {
		var inv model.Invoice
		if err := json.Unmarshal([]byte(r.data), &inv); err != nil {
			return nil, fmt.Errorf("store: unmarshal invoice %d: %w", r.id, err)
		}
		inv.ID = r.id
		out = append(out, inv)
	}
	return out, nil
}

// DeleteInvoice removes an invoice by ID.
func (d *DB) DeleteInvoice(id int64) error {
	_, err := d.sql.Exec(`DELETE FROM invoices WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete invoice %d: %w", id, err)
	}
	return nil
}
