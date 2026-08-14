package store

import (
	"encoding/json"
	"fmt"

	"dock-invoice/internal/model"
)

// SaveCustomer inserts a new customer master record and returns its ID.
func (d *DB) SaveCustomer(c model.Customer) (int64, error) {
	billing, err := json.Marshal(c.BillingAddress)
	if err != nil {
		return 0, fmt.Errorf("store: marshal billing address: %w", err)
	}
	shipping, err := json.Marshal(c.ShippingAddress)
	if err != nil {
		return 0, fmt.Errorf("store: marshal shipping address: %w", err)
	}
	res, err := d.sql.Exec(
		`INSERT INTO customers (name, gstin, billing_address, shipping_address) VALUES (?, ?, ?, ?)`,
		c.Name, c.GSTIN, string(billing), string(shipping),
	)
	if err != nil {
		return 0, fmt.Errorf("store: insert customer: %w", err)
	}
	return res.LastInsertId()
}

// UpdateCustomer overwrites an existing customer master record by ID.
func (d *DB) UpdateCustomer(c model.Customer) error {
	billing, err := json.Marshal(c.BillingAddress)
	if err != nil {
		return fmt.Errorf("store: marshal billing address: %w", err)
	}
	shipping, err := json.Marshal(c.ShippingAddress)
	if err != nil {
		return fmt.Errorf("store: marshal shipping address: %w", err)
	}
	_, err = d.sql.Exec(
		`UPDATE customers SET name = ?, gstin = ?, billing_address = ?, shipping_address = ? WHERE id = ?`,
		c.Name, c.GSTIN, string(billing), string(shipping), c.ID,
	)
	if err != nil {
		return fmt.Errorf("store: update customer %d: %w", c.ID, err)
	}
	return nil
}

// GetCustomer fetches a customer master record by ID.
func (d *DB) GetCustomer(id int64) (model.Customer, error) {
	row := d.sql.QueryRow(`SELECT id, name, gstin, billing_address, shipping_address FROM customers WHERE id = ?`, id)
	return scanCustomer(row)
}

// ListCustomers returns all customer master records ordered by name.
func (d *DB) ListCustomers() ([]model.Customer, error) {
	rows, err := d.sql.Query(`SELECT id, name, gstin, billing_address, shipping_address FROM customers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list customers: %w", err)
	}
	defer rows.Close()

	var out []model.Customer
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCustomer(row scanner) (model.Customer, error) {
	var c model.Customer
	var billing, shipping string
	if err := row.Scan(&c.ID, &c.Name, &c.GSTIN, &billing, &shipping); err != nil {
		return model.Customer{}, fmt.Errorf("store: scan customer: %w", err)
	}
	if err := json.Unmarshal([]byte(billing), &c.BillingAddress); err != nil {
		return model.Customer{}, fmt.Errorf("store: unmarshal billing address: %w", err)
	}
	if err := json.Unmarshal([]byte(shipping), &c.ShippingAddress); err != nil {
		return model.Customer{}, fmt.Errorf("store: unmarshal shipping address: %w", err)
	}
	return c, nil
}
