package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/testfixture"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	// A unique in-memory DB per test (":memory:" is fine since each Open
	// call gets its own connection pool/instance in modernc.org/sqlite).
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInvoiceRoundTrip(t *testing.T) {
	db := openTestDB(t)

	inv := model.Invoice{
		Type:          model.InvoiceExportLUT,
		Number:        "AEX2425-1",
		Date:          time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC),
		DueDate:       time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
		Customer:      testfixture.ExportCustomer(),
		PlaceOfSupply: "97-Other Territory",
		Currency:      "USD",
		FXFactor:      money.Rate(83_200_000),
		Items: []model.LineItem{
			{Description: "Software Development Services", HSNSAC: "998314", Quantity: money.Qty(100), Unit: "UNT", RateUSD: money.Amount(400000), TaxRatePct: 1800},
		},
		Seller: testfixture.SellerCompany(),
	}

	id, err := db.SaveInvoice(inv)
	if err != nil {
		t.Fatalf("SaveInvoice: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero invoice ID")
	}

	got, err := db.GetInvoice(id)
	if err != nil {
		t.Fatalf("GetInvoice: %v", err)
	}
	if got.Number != inv.Number {
		t.Errorf("Number = %q, want %q", got.Number, inv.Number)
	}
	if got.Customer.Name != testfixture.ExportCustomerName {
		t.Errorf("Customer.Name = %q, want %q", got.Customer.Name, testfixture.ExportCustomerName)
	}
	if len(got.Items) != 1 || got.Items[0].RateUSD != money.Amount(400000) {
		t.Errorf("Items round-trip mismatch: %+v", got.Items)
	}
	if got.FXFactor != inv.FXFactor {
		t.Errorf("FXFactor = %d, want %d", got.FXFactor, inv.FXFactor)
	}

	list, err := db.ListInvoices()
	if err != nil {
		t.Fatalf("ListInvoices: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListInvoices len = %d, want 1", len(list))
	}
}

// TestInvoiceSnapshotUnaffectedByCustomerEdit verifies the core legal
// requirement: an invoice stores a customer snapshot, so editing the
// customer master afterwards must not change what a previously-saved
// invoice reports.
func TestInvoiceSnapshotUnaffectedByCustomerEdit(t *testing.T) {
	db := openTestDB(t)

	custID, err := db.SaveCustomer(testfixture.ExportCustomer())
	if err != nil {
		t.Fatalf("SaveCustomer: %v", err)
	}

	cust, err := db.GetCustomer(custID)
	if err != nil {
		t.Fatalf("GetCustomer: %v", err)
	}

	inv := model.Invoice{
		Type:     model.InvoiceExportLUT,
		Number:   "AEX2425-1",
		Date:     time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC),
		Customer: cust, // snapshot taken at save time
	}
	invID, err := db.SaveInvoice(inv)
	if err != nil {
		t.Fatalf("SaveInvoice: %v", err)
	}

	// Now edit the customer master record.
	cust.Name = testfixture.ExportCustomerName + " (RENAMED)"
	cust.BillingAddress = []string{"Austin", "Texas", "United States of America"}
	if err := db.UpdateCustomer(cust); err != nil {
		t.Fatalf("UpdateCustomer: %v", err)
	}

	// The saved invoice must still show the original snapshot.
	got, err := db.GetInvoice(invID)
	if err != nil {
		t.Fatalf("GetInvoice: %v", err)
	}
	if got.Customer.Name != testfixture.ExportCustomerName {
		t.Errorf("invoice snapshot changed after customer edit: Name = %q, want original %q", got.Customer.Name, testfixture.ExportCustomerName)
	}
	if len(got.Customer.BillingAddress) != 3 || got.Customer.BillingAddress[0] != "1209 Orange Street" {
		t.Errorf("invoice snapshot billing address changed after customer edit: %+v", got.Customer.BillingAddress)
	}

	// Sanity: the master record itself really was updated.
	updated, err := db.GetCustomer(custID)
	if err != nil {
		t.Fatalf("GetCustomer after update: %v", err)
	}
	if updated.Name != testfixture.ExportCustomerName+" (RENAMED)" {
		t.Errorf("customer master not updated: %q", updated.Name)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	db := openTestDB(t)

	_, ok, err := db.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if ok {
		t.Fatal("expected no settings saved yet")
	}

	s := model.Settings{
		Company:      testfixture.SellerCompany(),
		Bank:         testfixture.SellerBank(),
		LastFXFactor: "83.2",
	}
	if err := db.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	got, ok, err := db.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !ok {
		t.Fatal("expected settings to be found")
	}
	if got.Company.Name != testfixture.SellerName {
		t.Errorf("Company.Name = %q, want %q", got.Company.Name, testfixture.SellerName)
	}
	if got.LastFXFactor != "83.2" {
		t.Errorf("LastFXFactor = %q, want 83.2", got.LastFXFactor)
	}

	// Upsert should overwrite, not error.
	s.LastFXFactor = "84.0"
	if err := db.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings (update): %v", err)
	}
	got2, _, err := db.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after update: %v", err)
	}
	if got2.LastFXFactor != "84.0" {
		t.Errorf("LastFXFactor after update = %q, want 84.0", got2.LastFXFactor)
	}
}

func TestItemRoundTrip(t *testing.T) {
	db := openTestDB(t)

	id, err := db.SaveItem(model.Item{Description: "Consulting", HSNSAC: "998314", Unit: "HRS", DefaultRate: money.Amount(10000), Currency: model.CurrencyINR})
	if err != nil {
		t.Fatalf("SaveItem: %v", err)
	}

	items, err := db.ListItems()
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 || items[0].ID != id || items[0].DefaultRate != money.Amount(10000) || items[0].Currency != model.CurrencyINR {
		t.Errorf("ListItems = %+v", items)
	}
}

// TestItemDefaultsToUSDCurrency covers the documented default for a saved
// item that never had a currency set explicitly (e.g. a struct literal
// built before this field existed) — it must not silently become an empty
// string that formatItemRate/rateForInvoiceCurrency would then mishandle.
func TestItemDefaultsToUSDCurrency(t *testing.T) {
	db := openTestDB(t)

	id, err := db.SaveItem(model.Item{Description: "Legacy item", DefaultRate: money.Amount(5000)})
	if err != nil {
		t.Fatalf("SaveItem: %v", err)
	}

	items, err := db.ListItems()
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	var got model.Item
	for _, it := range items {
		if it.ID == id {
			got = it
		}
	}
	if got.Currency != model.CurrencyUSD {
		t.Errorf("Currency for item saved without one = %q, want %q", got.Currency, model.CurrencyUSD)
	}
}

// TestMigrateItemsCurrency_AddsColumnToPreExistingTable simulates opening a
// database created before model.Item.Currency existed (bare rate_usd
// column, no currency column) and checks Open's migration backfills
// 'USD' for every pre-existing row without touching rate_usd.
func TestMigrateItemsCurrency_AddsColumnToPreExistingTable(t *testing.T) {
	path := t.TempDir() + "/legacy.db"

	// Build a database with the old (pre-currency) items schema directly,
	// bypassing Open/schema.sql so this test exercises exactly the
	// upgrade path a real pre-existing user database would hit.
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := legacy.Exec(`CREATE TABLE items (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		description TEXT NOT NULL,
		hsn_sac     TEXT NOT NULL DEFAULT '',
		unit        TEXT NOT NULL DEFAULT '',
		rate_usd    INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatalf("create legacy items table: %v", err)
	}
	if _, err := legacy.Exec(`INSERT INTO items (description, hsn_sac, unit, rate_usd) VALUES ('Old item', '998314', 'UNT', 400000)`); err != nil {
		t.Fatalf("seed legacy item: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open (migration): %v", err)
	}
	defer db.Close()

	items, err := db.ListItems()
	if err != nil {
		t.Fatalf("ListItems after migration: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListItems after migration = %d items, want 1", len(items))
	}
	if items[0].Currency != model.CurrencyUSD {
		t.Errorf("migrated item Currency = %q, want %q (the pre-migration assumption)", items[0].Currency, model.CurrencyUSD)
	}
	if items[0].DefaultRate != money.Amount(400000) {
		t.Errorf("migrated item DefaultRate = %d, want 400000 (unchanged by the migration)", items[0].DefaultRate)
	}
}

func TestNextInvoiceNumber_GoldenAndSequence(t *testing.T) {
	db := openTestDB(t)

	d := time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC)
	n1, err := db.NextInvoiceNumber("AEX{FY}-{SEQ}", d)
	if err != nil {
		t.Fatalf("NextInvoiceNumber: %v", err)
	}
	if n1 != "AEX2425-1" {
		t.Errorf("first number = %q, want AEX2425-1", n1)
	}

	n2, err := db.NextInvoiceNumber("AEX{FY}-{SEQ}", d)
	if err != nil {
		t.Fatalf("NextInvoiceNumber: %v", err)
	}
	if n2 != "AEX2425-2" {
		t.Errorf("second number = %q, want AEX2425-2", n2)
	}

	// A different pattern/series is independent and starts at 1.
	n3, err := db.NextInvoiceNumber("DOM{FY}-{SEQ}", d)
	if err != nil {
		t.Fatalf("NextInvoiceNumber: %v", err)
	}
	if n3 != "DOM2425-1" {
		t.Errorf("other series number = %q, want DOM2425-1", n3)
	}

	// FY rollover on 1 April resets the AEX series counter to 1.
	next := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
	n4, err := db.NextInvoiceNumber("AEX{FY}-{SEQ}", next)
	if err != nil {
		t.Fatalf("NextInvoiceNumber: %v", err)
	}
	if n4 != "AEX2526-1" {
		t.Errorf("post-rollover number = %q, want AEX2526-1", n4)
	}
}

func TestBackupTo_WritesConsistentSnapshot(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.db")
	backupPath := filepath.Join(dir, "backup.db")

	db, err := Open(srcPath)
	if err != nil {
		t.Fatalf("Open source: %v", err)
	}
	if _, err := db.SaveCustomer(model.Customer{Name: "Backup Test Co"}); err != nil {
		t.Fatalf("SaveCustomer: %v", err)
	}
	if err := db.BackupTo(backupPath); err != nil {
		t.Fatalf("BackupTo: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close source: %v", err)
	}

	restored, err := Open(backupPath)
	if err != nil {
		t.Fatalf("Open backup: %v", err)
	}
	defer restored.Close()

	customers, err := restored.ListCustomers()
	if err != nil {
		t.Fatalf("ListCustomers: %v", err)
	}
	if len(customers) != 1 || customers[0].Name != "Backup Test Co" {
		t.Fatalf("restored customers = %+v, want one 'Backup Test Co'", customers)
	}
}
