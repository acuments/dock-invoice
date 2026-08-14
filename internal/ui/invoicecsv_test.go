package ui

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

// csvCell reads the value of a named column out of a record, so the
// assertions below name the column they mean instead of an index that
// changes every time invoiceCSVHeader grows a field.
func csvCell(t *testing.T, rec []string, column string) string {
	t.Helper()
	for i, name := range invoiceCSVHeader {
		if name == column {
			if i >= len(rec) {
				t.Fatalf("record has %d fields, no value for column %q at index %d", len(rec), column, i)
			}
			return rec[i]
		}
	}
	t.Fatalf("unknown column %q", column)
	return ""
}

// mkDomesticInvoice is an intra-state domestic invoice (seller and buyer both
// in state 33), which is what makes calc split the tax into CGST+SGST. The
// FX factor is 1.0 because line rates are always entered in USD, even on a
// rupee invoice — a zero factor would convert every taxable value to nothing.
func mkDomesticInvoice(number string, date time.Time) model.Invoice {
	return model.Invoice{
		Type:          model.InvoiceDomestic,
		Number:        number,
		ReferenceNo:   "PO-9",
		Date:          date,
		Seller:        model.Company{GSTIN: "33AAECA1234A1Z9", StateCode: "33"},
		Customer:      model.Customer{Name: "Acme Corp", GSTIN: "33AAAAA0000A1Z5"},
		PlaceOfSupply: "33-Tamil Nadu",
		Currency:      model.CurrencyINR,
		FXFactor:      money.Rate(money.RateScale),
		Items: []model.LineItem{
			{Description: "Consulting", Quantity: money.Qty(100), RateUSD: 100000, TaxRatePct: 1800},
		},
	}
}

// mkExportInvoice is an LUT export: priced in USD, no tax charged.
func mkExportInvoice(number string, date time.Time) model.Invoice {
	return model.Invoice{
		Type:     model.InvoiceExportLUT,
		Number:   number,
		Date:     date,
		Customer: model.Customer{Name: "Northwind Trading Co."},
		Currency: model.CurrencyUSD,
		FXFactor: money.Rate(83 * money.RateScale),
		Items: []model.LineItem{
			{Description: "Software", Quantity: money.Qty(100), RateUSD: 100000, TaxRatePct: 1800},
		},
	}
}

// ---- figures must survive as figures ------------------------------------

// A spreadsheet is the entire destination here, so an exported amount must
// parse as a number: no ₹/$ symbol, no Indian digit grouping, no em-dash.
func TestCSVAmount_IsPlainDecimal(t *testing.T) {
	cases := []struct {
		in   money.Amount
		want string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{100, "1.00"},
		{11800000, "118000.00"},   // would print as "1,18,000.00" in the UI
		{-50, "-0.50"},            // a domestic round-off can go either way
		{-11800000, "-118000.00"}, // and the minus must lead the whole number
	}
	for _, c := range cases {
		if got := csvAmount(c.in); got != c.want {
			t.Errorf("csvAmount(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInvoiceCSVRow_NoCurrencySymbolsOrGrouping(t *testing.T) {
	rec := invoiceCSVRow(mkDomesticInvoice("DOM-1", time.Date(2025, time.March, 4, 0, 0, 0, 0, time.UTC)))
	for i, cell := range rec {
		if strings.ContainsAny(cell, "₹$—") {
			t.Errorf("column %q carries UI formatting: %q", invoiceCSVHeader[i], cell)
		}
	}
	if got := csvCell(t, rec, "Total (INR)"); strings.Contains(got, ",") {
		t.Errorf("Total (INR) = %q, want an ungrouped decimal", got)
	}
}

// ISO dates, not the list's "04 Mar 2025": a spreadsheet parses these the
// same way regardless of the machine's locale.
func TestInvoiceCSVRow_DateIsISO(t *testing.T) {
	rec := invoiceCSVRow(mkDomesticInvoice("DOM-1", time.Date(2025, time.March, 4, 0, 0, 0, 0, time.UTC)))
	if got := csvCell(t, rec, "Date"); got != "2025-03-04" {
		t.Errorf("Date = %q, want %q", got, "2025-03-04")
	}
}

// The exported type must be the display label, never the stored enum — the
// same rule the on-screen table follows.
func TestInvoiceCSVRow_TypeIsDisplayLabel(t *testing.T) {
	rec := invoiceCSVRow(mkDomesticInvoice("DOM-1", time.Now()))
	if got := csvCell(t, rec, "Type"); got != "Domestic" {
		t.Errorf("Type = %q, want %q", got, "Domestic")
	}
	if got := csvCell(t, mkExportRow(), "Type"); got != "Export (LUT)" {
		t.Errorf("Type = %q, want %q", got, "Export (LUT)")
	}
}

func mkExportRow() []string {
	return invoiceCSVRow(mkExportInvoice("AEX-1", time.Now()))
}

// A domestic invoice carries the CGST/SGST split and, since it is not raised
// in dollars, blank USD columns — blank rather than "0.00", so a total taken
// over the USD column never quietly counts rupee invoices as zero-dollar
// ones.
func TestInvoiceCSVRow_DomesticSplitAndBlankUSD(t *testing.T) {
	rec := invoiceCSVRow(mkDomesticInvoice("DOM-1", time.Now()))

	// One unit at 1000.00, converted at 1.0, taxed at 18% split evenly.
	if got := csvCell(t, rec, "Taxable (INR)"); got != "1000.00" {
		t.Errorf("Taxable (INR) = %q, want %q", got, "1000.00")
	}
	cgst, sgst := csvCell(t, rec, "CGST (INR)"), csvCell(t, rec, "SGST (INR)")
	if cgst != "90.00" || sgst != "90.00" {
		t.Errorf("CGST/SGST = %q/%q, want 90.00 each", cgst, sgst)
	}
	if got := csvCell(t, rec, "IGST (INR)"); got != "0.00" {
		t.Errorf("IGST (INR) = %q; an inapplicable tax is still a charged-nil figure, want %q", got, "0.00")
	}
	if got := csvCell(t, rec, "Total (INR)"); got != "1180.00" {
		t.Errorf("Total (INR) = %q, want %q", got, "1180.00")
	}
	for _, col := range []string{"Taxable (USD)", "Total (USD)"} {
		if got := csvCell(t, rec, col); got != "" {
			t.Errorf("%s = %q on an INR invoice, want blank", col, got)
		}
	}
}

// An export invoice is the mirror image: the USD leg is populated, and an LUT
// export's zero tax exports as a real 0.00 rather than a blank.
func TestInvoiceCSVRow_ExportCarriesUSDLeg(t *testing.T) {
	rec := invoiceCSVRow(mkExportInvoice("AEX-1", time.Now()))

	if got := csvCell(t, rec, "Total (USD)"); got != "1000.00" {
		t.Errorf("Total (USD) = %q, want %q", got, "1000.00")
	}
	if got := csvCell(t, rec, "Total (INR)"); got != "83000.00" {
		t.Errorf("Total (INR) = %q, want the FX-converted value %q", got, "83000.00")
	}
	if got := csvCell(t, rec, "IGST (INR)"); got != "0.00" {
		t.Errorf("IGST (INR) = %q, want %q under LUT", got, "0.00")
	}
	if got := csvCell(t, rec, "FX Factor"); got != "83" {
		t.Errorf("FX Factor = %q, want %q", got, "83")
	}
}

// ---- the file itself ----------------------------------------------------

func TestWriteInvoiceCSV_HeaderAndOrder(t *testing.T) {
	invs := []model.Invoice{
		mkExportInvoice("AEX-3", time.Now()),
		mkDomesticInvoice("DOM-1", time.Now()),
		mkExportInvoice("AEX-2", time.Now()),
	}

	var buf bytes.Buffer
	if err := writeInvoiceCSV(&buf, invs); err != nil {
		t.Fatalf("writeInvoiceCSV: %v", err)
	}
	recs, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("re-read exported CSV: %v", err)
	}

	if len(recs) != len(invs)+1 {
		t.Fatalf("got %d records, want %d (header + %d invoices)", len(recs), len(invs)+1, len(invs))
	}
	if !equalStrings(recs[0], invoiceCSVHeader) {
		t.Errorf("header = %v, want %v", recs[0], invoiceCSVHeader)
	}
	// Row order must be the list's order — the user's chosen sort — not the
	// store's or anything re-derived here.
	for i, inv := range invs {
		if got := csvCell(t, recs[i+1], "Number"); got != inv.Number {
			t.Errorf("row %d Number = %q, want %q", i, got, inv.Number)
		}
	}
}

// Commas and quotes inside a customer name must not tear the row apart —
// the reason this goes through encoding/csv rather than a hand-joined string.
func TestWriteInvoiceCSV_QuotesSeparatorsInNames(t *testing.T) {
	inv := mkExportInvoice("AEX-1", time.Now())
	inv.Customer.Name = `Widgets, "Inc." Pvt Ltd`

	var buf bytes.Buffer
	if err := writeInvoiceCSV(&buf, []model.Invoice{inv}); err != nil {
		t.Fatalf("writeInvoiceCSV: %v", err)
	}
	recs, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("re-read exported CSV: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want header + 1 row", len(recs))
	}
	if got := csvCell(t, recs[1], "Customer"); got != inv.Customer.Name {
		t.Errorf("Customer = %q, want %q", got, inv.Customer.Name)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- the suggested filename ---------------------------------------------

func TestInvoiceCSVFilename_DescribesTheFilters(t *testing.T) {
	now := time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		f    invoiceFilter
		want string
	}{
		{"unfiltered falls back to the date", invoiceFilter{}, "invoices-2026-08-13.csv"},
		{"fy only", invoiceFilter{fy: "2425"}, "invoices-fy2024-25.csv"},
		{"month only", invoiceFilter{month: time.March}, "invoices-march.csv"},
		{"fy and month", invoiceFilter{fy: "2425", month: time.March}, "invoices-fy2024-25-march.csv"},
		{"a query contributes a word, never its text", invoiceFilter{query: "Acme/Corp"}, "invoices-search.csv"},
		{"whitespace-only query is no filter at all", invoiceFilter{query: "   "}, "invoices-2026-08-13.csv"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := invoiceCSVFilename(c.f, now); got != c.want {
				t.Errorf("invoiceCSVFilename = %q, want %q", got, c.want)
			}
		})
	}
}

// The name must never inherit characters from the query, which is free text
// and can hold path separators.
func TestInvoiceCSVFilename_NeverEmbedsQueryText(t *testing.T) {
	got := invoiceCSVFilename(invoiceFilter{query: "../../etc/passwd"}, time.Now())
	if strings.ContainsAny(got, `/\`) {
		t.Errorf("filename %q carries path separators from the search text", got)
	}
}

// ---- the screen wiring ---------------------------------------------------

// exportedNumbers runs the screen's real export selection (exportRows, the
// same call exportCSV makes) through the real writer, and reads the invoice
// numbers back out of the resulting file. Going through the file rather than
// asserting on the slice is deliberate: it is the file the user opens, and it
// is the only place a bug between "which rows" and "what got written" can
// still be caught.
func exportedNumbers(t *testing.T, s *InvoicesScreen) []string {
	t.Helper()
	var buf bytes.Buffer
	if err := writeInvoiceCSV(&buf, s.exportRows()); err != nil {
		t.Fatalf("writeInvoiceCSV: %v", err)
	}
	recs, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("re-read exported CSV: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("exported file has no header row")
	}
	if !equalStrings(recs[0], invoiceCSVHeader) {
		t.Fatalf("header = %v, want %v", recs[0], invoiceCSVHeader)
	}
	nums := make([]string, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		nums = append(nums, csvCell(t, rec, "Number"))
	}
	return nums
}

// Each filter control on its own, and all three together, must scope the
// file — this is the whole promise of the feature, and the one thing a user
// filing GSTR-1 for a single month cannot verify by eye once the file is
// open.
func TestInvoicesScreen_ExportScopedToEachFilter(t *testing.T) {
	// Two financial years (April boundary), several months, two customers.
	seed := []struct {
		number, customer string
		date             time.Time
	}{
		{"AEX2324-1", "Acme Corp", time.Date(2023, time.June, 5, 0, 0, 0, 0, time.UTC)},    // FY 2023-24
		{"AEX2425-1", "Acme Corp", time.Date(2024, time.May, 2, 0, 0, 0, 0, time.UTC)},     // FY 2024-25
		{"AEX2425-2", "Beta LLC", time.Date(2024, time.May, 9, 0, 0, 0, 0, time.UTC)},      // FY 2024-25
		{"AEX2425-3", "Acme Corp", time.Date(2025, time.January, 8, 0, 0, 0, 0, time.UTC)}, // still FY 2024-25
	}

	cases := []struct {
		name  string
		apply func(s *InvoicesScreen)
		want  []string
	}{
		{
			"no filter exports everything",
			func(s *InvoicesScreen) {},
			[]string{"AEX2324-1", "AEX2425-1", "AEX2425-2", "AEX2425-3"},
		},
		{
			"search scopes to the customer",
			func(s *InvoicesScreen) { s.searchEntry.SetText("Beta") },
			[]string{"AEX2425-2"},
		},
		{
			"FY scopes across the April boundary",
			func(s *InvoicesScreen) { s.fySelect.SetSelected(fyLabel("2425")) },
			[]string{"AEX2425-1", "AEX2425-2", "AEX2425-3"},
		},
		{
			"month scopes across financial years",
			func(s *InvoicesScreen) { s.monthSelect.SetSelected(time.May.String()) },
			[]string{"AEX2425-1", "AEX2425-2"},
		},
		{
			"all three narrow together",
			func(s *InvoicesScreen) {
				s.searchEntry.SetText("Acme")
				s.fySelect.SetSelected(fyLabel("2425"))
				s.monthSelect.SetSelected(time.May.String())
			},
			[]string{"AEX2425-1"},
		},
		{
			"clearing the filters restores the full set",
			func(s *InvoicesScreen) {
				s.searchEntry.SetText("Beta")
				s.clearFilters()
			},
			[]string{"AEX2324-1", "AEX2425-1", "AEX2425-2", "AEX2425-3"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, db := newTestInvoicesScreen(t)
			for _, sd := range seed {
				seedInvoice(t, db, sd.number, sd.customer, sd.date)
			}
			s.Reload()
			c.apply(s)

			got := exportedNumbers(t, s)
			if !equalSets(got, c.want) {
				t.Errorf("exported %v, want %v", got, c.want)
			}
		})
	}
}

// The export must cover the filtered set — every match, across every page —
// rather than the fifteen rows the list happens to be showing. An export that
// silently stopped at a page boundary would be the failure mode most likely
// to go unnoticed, since the file looks perfectly well-formed either way.
func TestInvoicesScreen_ExportCoversFilteredSetAcrossPages(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	s.pageSize = 2
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-2", "Beta LLC", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-3", "Acme Corp", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-4", "Acme Corp", time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC))
	s.Reload()

	s.searchEntry.SetText("Acme")
	if got, want := s.pageLength(), 2; got != want {
		t.Fatalf("page length = %d, want %d — the test needs the matches to span pages", got, want)
	}

	got := exportedNumbers(t, s)
	if want := []string{"AEX-1", "AEX-3", "AEX-4"}; !equalSets(got, want) {
		t.Errorf("exported %v, want %v — every match, not just the visible page", got, want)
	}
}

// A column sort is part of what the user set up before hitting Export, so the
// file must arrive in that order rather than reverting to the store's.
func TestInvoicesScreen_ExportFollowsColumnSort(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-2", "Beta LLC", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-3", "Contoso", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	s.Reload()

	s.handleHeaderClick(colNumber) // ascending on first click
	if got, want := exportedNumbers(t, s), []string{"AEX-1", "AEX-2", "AEX-3"}; !equalStrings(got, want) {
		t.Errorf("exported %v, want %v", got, want)
	}

	s.handleHeaderClick(colNumber) // same column again reverses
	if got, want := exportedNumbers(t, s), []string{"AEX-3", "AEX-2", "AEX-1"}; !equalStrings(got, want) {
		t.Errorf("exported %v after reversing the sort, want %v", got, want)
	}
}

// The rows are snapshotted when Export is clicked, not when the file dialog
// is answered. A filter changed while that dialog is open must not alter what
// gets written.
func TestInvoicesScreen_ExportRowsAreSnapshotted(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-2", "Beta LLC", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	s.Reload()

	s.searchEntry.SetText("Acme")
	rows := s.exportRows() // as exportCSV does, before opening the dialog

	// The user fiddles with the filter while the save sheet is up.
	s.searchEntry.SetText("Beta")

	if len(rows) != 1 || rows[0].Number != "AEX-1" {
		t.Fatalf("snapshot changed under a later filter: %v", numbersOf(rows))
	}
}

// equalSets compares without imposing an order, for the scoping cases where
// membership is the claim and ordering is covered separately.
func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	return true
}

// Export is only offered when it would produce rows: disabled on an empty
// database, and disabled again the moment a filter matches nothing.
func TestInvoicesScreen_ExportButtonFollowsWhatIsExportable(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	if !s.exportBtn.Disabled() {
		t.Error("export should start disabled with no invoices in the database")
	}

	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	s.Reload()
	if s.exportBtn.Disabled() {
		t.Error("export should be enabled once there is an invoice to export")
	}

	s.searchEntry.SetText("nobody by this name")
	if !s.exportBtn.Disabled() {
		t.Error("export should be disabled while the filter matches nothing")
	}

	s.clearFilters()
	if s.exportBtn.Disabled() {
		t.Error("export should be re-enabled after the filter is cleared")
	}
}

func TestInvoiceExportLabel_Pluralises(t *testing.T) {
	if got := invoiceExportLabel(1, "/tmp/x.csv"); got != "Exported 1 invoice to /tmp/x.csv" {
		t.Errorf("got %q", got)
	}
	if got := invoiceExportLabel(34, "/tmp/x.csv"); got != "Exported 34 invoices to /tmp/x.csv" {
		t.Errorf("got %q", got)
	}
}
