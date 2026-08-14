package ui

import (
	"reflect"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/store"
)

// newTestInvoicesScreen wires an InvoicesScreen against a real in-memory
// store, following the pattern in app_test.go's newTestApp:
// test.NewTempApp(t) + store.Open(":memory:"), never NewApp/a real Fyne
// window, which would try to open an actual OS window and hang here.
func newTestInvoicesScreen(t *testing.T) (*InvoicesScreen, *store.DB) {
	t.Helper()
	test.NewTempApp(t)
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	s := NewInvoicesScreen(nil, db,
		func() model.Settings { return model.Settings{} },
		func(*EditorState) {},
		func(model.Invoice) (string, error) { return "", nil },
	)
	return s, db
}

// seedInvoice saves a minimal-but-valid invoice with the given number,
// customer name and date, for tests that only care about ordering/labels
// rather than tax arithmetic.
func seedInvoice(t *testing.T, db *store.DB, number, customer string, date time.Time) int64 {
	t.Helper()
	id, err := db.SaveInvoice(model.Invoice{
		Type:     model.InvoiceExportLUT,
		Number:   number,
		Date:     date,
		Customer: model.Customer{Name: customer},
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("SaveInvoice(%q): %v", number, err)
	}
	return id
}

func numbersOf(invs []model.Invoice) []string {
	out := make([]string, len(invs))
	for i, inv := range invs {
		out[i] = inv.Number
	}
	return out
}

func containsText(texts []string, want string) bool {
	for _, t := range texts {
		if t == want {
			return true
		}
	}
	return false
}

// ---- C1: raw enum values must never reach the UI -------------------------

func TestInvoiceTypeLabel_KnownAndUnknown(t *testing.T) {
	cases := []struct {
		in   model.InvoiceType
		want string
	}{
		{model.InvoiceExportLUT, "Export (LUT)"},
		{model.InvoiceExportIGST, "Export (IGST)"},
		{model.InvoiceDomestic, "Domestic"},
		{"", "Unknown"},                                   // zero value: render a visible placeholder, not a blank cell
		{model.InvoiceType("future_type"), "future_type"}, // unrecognised: fall back to raw rather than hide it
	}
	for _, c := range cases {
		if got := InvoiceTypeLabel(c.in); got != c.want {
			t.Errorf("InvoiceTypeLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInvoiceTypeFromLabel_RoundTrips(t *testing.T) {
	for _, want := range []model.InvoiceType{model.InvoiceExportLUT, model.InvoiceExportIGST, model.InvoiceDomestic} {
		label := InvoiceTypeLabel(want)
		got, ok := InvoiceTypeFromLabel(label)
		if !ok || got != want {
			t.Errorf("InvoiceTypeFromLabel(%q) = (%q, %v), want (%q, true)", label, got, ok, want)
		}
	}
	if _, ok := InvoiceTypeFromLabel("not a real label"); ok {
		t.Error("expected ok=false for an unrecognised label")
	}
}

// TestInvoicesScreen_TableShowsHumanReadableType is the regression test for
// bug C1: the invoices table used to render model.Invoice.Type's raw
// serialised value (e.g. "export_lut") straight into the "Type" column.
func TestInvoicesScreen_TableShowsHumanReadableType(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Now())
	s.Reload()

	win := test.NewWindow(s.Container())
	defer win.Close()
	win.Resize(fyne.NewSize(1000, 400))
	win.Canvas().Capture() // force a render pass so the table populates its cells

	// Collect both widget kinds that can carry visible text: the table's cells
	// render with canvas.Text (so a cell can set its own weight, colour and
	// alignment), while the rest of the screen still uses widget.Label. The
	// assertion below is about the *words* rendered, not which widget renders
	// them, so both are gathered.
	var texts []string
	walk(s.Container(), func(o any) {
		switch v := o.(type) {
		case *widget.Label:
			texts = append(texts, v.Text)
		case *canvas.Text:
			texts = append(texts, v.Text)
		}
	})

	if !containsText(texts, "Export (LUT)") {
		t.Errorf("expected the table to render the human label %q; got labels %v", "Export (LUT)", texts)
	}
	if containsText(texts, "export_lut") {
		t.Errorf("table rendered the raw enum value \"export_lut\" instead of a human label: %v", texts)
	}
}

// ---- C2: stale selection must not survive a Reload ------------------------

// TestInvoicesScreen_ReloadClearsStaleSelection is the regression test for
// bug C2: Reload used to reset s.selected to -1 without calling
// s.list.UnselectAll(), so the list widget kept drawing a row as selected
// even though the screen itself considered nothing selected.
func TestInvoicesScreen_ReloadClearsStaleSelection(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Now())
	s.Reload()

	s.list.OnSelected(0)
	if s.selectedInvoice() == nil {
		t.Fatal("expected an invoice to be selected before Reload")
	}

	s.Reload()

	if s.selected != -1 {
		t.Errorf("selected = %d after Reload, want -1", s.selected)
	}
	if s.selectedInvoice() != nil {
		t.Error("selectedInvoice() should be nil right after Reload")
	}
	// The user-visible symptom from the bug report: with a stale
	// selection, deleteSelected either silently no-ops or (as here, once
	// s.selected is reset but the table cell still looks highlighted)
	// reports the "select an invoice" message while a row still appears
	// selected on screen. Confirm the message fires correctly now that
	// selection state is consistent.
	s.deleteSelected()
	if got, want := s.status.Text, "Select an invoice to delete."; got != want {
		t.Errorf("status after deleteSelected() with no real selection = %q, want %q", got, want)
	}
}

// TestInvoicesScreen_HeaderRowIsAlwaysVisible guards the property the old
// StickyRowCount assertion was really about: the column captions must not
// scroll away with the rows. The header used to be row zero of a widget.Table
// pinned via StickyRowCount; it is now built outside the scrolling list
// entirely (see buildListHeader), which gets the same guarantee structurally.
// We cannot read pixels from a unit test, so we assert the header cells exist
// and are not part of the list's own content.
func TestInvoicesScreen_HeaderRowIsAlwaysVisible(t *testing.T) {
	s, _ := newTestInvoicesScreen(t)
	if len(s.headerCells) != len(invoiceListColumns) {
		t.Fatalf("headerCells = %d, want one per column (%d)", len(s.headerCells), len(invoiceListColumns))
	}
	// The list reports only the current page's data rows; a header row inside
	// it would show up here as an extra item and start scrolling with the
	// data again. Pagination length (not the full filtered set) is what the
	// list itself knows about.
	if got := s.list.Length(); got != s.pageLength() {
		t.Errorf("list length = %d, want %d (current page only, header lives outside the list)", got, s.pageLength())
	}
}

// ---- C3: user-controlled sorting on top of a sensible default -------------

// TestInvoicesScreen_DefaultOrderIsStoreOrder checks that, absent any
// header click, the screen leaves ListInvoices' own ordering (most
// recently created first) alone rather than silently re-sorting it.
func TestInvoicesScreen_DefaultOrderIsStoreOrder(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "A-1", "Alice", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "B-1", "Bob", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "C-1", "Carol", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	s.Reload()

	if s.sortCol != -1 {
		t.Fatalf("sortCol = %d, want -1 (no user override) before any header click", s.sortCol)
	}
	want := []string{"C-1", "B-1", "A-1"} // most recently inserted (highest id) first
	if got := numbersOf(s.invoices); !reflect.DeepEqual(got, want) {
		t.Errorf("default order = %v, want %v", got, want)
	}
}

// TestInvoicesScreen_HeaderClickSortsAndToggles is the regression/feature
// test for bug C3: clicking a column header sorts by that column, and
// clicking the same header again reverses the direction.
func TestInvoicesScreen_HeaderClickSortsAndToggles(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "C-1", "Carol", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "A-1", "Alice", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "B-1", "Bob", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	s.Reload()

	// Column 1 is "Number": a text column defaults to ascending.
	s.handleHeaderClick(1)
	if s.sortCol != 1 || !s.sortAsc {
		t.Fatalf("after first click on Number: sortCol=%d sortAsc=%v, want col=1 asc=true", s.sortCol, s.sortAsc)
	}
	if got, want := numbersOf(s.invoices), []string{"A-1", "B-1", "C-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ascending sort by Number = %v, want %v", got, want)
	}

	// Clicking the same header again reverses direction.
	s.handleHeaderClick(1)
	if s.sortAsc {
		t.Fatal("second click on the same header should toggle to descending")
	}
	if got, want := numbersOf(s.invoices), []string{"C-1", "B-1", "A-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("descending sort by Number = %v, want %v", got, want)
	}

	// Column 0 is "Date": a date column defaults to descending (newest
	// first), independent of insertion order.
	s.handleHeaderClick(0)
	if s.sortCol != 0 || s.sortAsc {
		t.Fatalf("after first click on Date: sortCol=%d sortAsc=%v, want col=0 asc=false", s.sortCol, s.sortAsc)
	}
	if got, want := numbersOf(s.invoices), []string{"B-1", "A-1", "C-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("descending sort by Date = %v, want %v", got, want)
	}
}

// TestInvoicesScreen_HeaderClickClearsSelection checks that sorting, which
// reorders s.invoices under whatever row index was previously selected,
// drops the stale selection rather than leaving it pointing at a different
// invoice than the one the user actually clicked.
func TestInvoicesScreen_HeaderClickClearsSelection(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "A-1", "Alice", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "B-1", "Bob", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	s.Reload()

	s.list.OnSelected(0)
	if s.selected != 0 {
		t.Fatalf("selected = %d, want 0", s.selected)
	}

	s.handleHeaderClick(2) // click the "Customer" header
	if s.selected != -1 {
		t.Errorf("selected = %d after a header click reordered the rows, want -1", s.selected)
	}
}

// ---- C4: masters panes need an empty state and must not go stale ---------

func newTestMastersScreen(t *testing.T) (*MastersScreen, *store.DB) {
	t.Helper()
	test.NewTempApp(t)
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewMastersScreen(nil, db), db
}

// TestMastersScreen_EmptyStateTogglesWithData is the regression test for
// bug C4's blank-rectangle problem: a fresh database should show an
// explanatory empty state in both panes, which should disappear again once
// there is real data to show instead.
func TestMastersScreen_EmptyStateTogglesWithData(t *testing.T) {
	m, db := newTestMastersScreen(t)

	if !m.customerEmpty.Visible() {
		t.Error("customerEmpty should be visible when there are no customers")
	}
	if !m.itemEmpty.Visible() {
		t.Error("itemEmpty should be visible when there are no saved items")
	}

	if _, err := db.SaveCustomer(model.Customer{Name: "Acme Corp"}); err != nil {
		t.Fatalf("SaveCustomer: %v", err)
	}
	if _, err := db.SaveItem(model.Item{Description: "Consulting"}); err != nil {
		t.Fatalf("SaveItem: %v", err)
	}
	m.reloadCustomers()
	m.reloadItems()

	if m.customerEmpty.Visible() {
		t.Error("customerEmpty should hide once a customer exists")
	}
	if m.itemEmpty.Visible() {
		t.Error("itemEmpty should hide once a saved item exists")
	}
}

// TestMastersScreen_ReloadClearsStaleSelection is the regression test for
// C4's other bug: reloadCustomers/reloadItems refreshed the underlying
// slice without resetting m.selectedCust/m.selectedItem or unselecting the
// widget.List, so a saved index could keep pointing at a row that moved
// (customers are listed "ORDER BY name") or no longer exists.
func TestMastersScreen_ReloadClearsStaleSelection(t *testing.T) {
	m, db := newTestMastersScreen(t)

	if _, err := db.SaveCustomer(model.Customer{Name: "Zebra Inc"}); err != nil {
		t.Fatalf("SaveCustomer: %v", err)
	}
	m.reloadCustomers()
	m.customerList.OnSelected(0)
	if m.selectedCust != 0 {
		t.Fatalf("selectedCust = %d, want 0", m.selectedCust)
	}

	// Insert a customer that sorts before "Zebra Inc"; after the reload,
	// index 0 in the refreshed list is a different customer than the one
	// that was selected.
	if _, err := db.SaveCustomer(model.Customer{Name: "Aardvark Co"}); err != nil {
		t.Fatalf("SaveCustomer: %v", err)
	}
	m.reloadCustomers()

	if m.selectedCust != -1 {
		t.Errorf("selectedCust = %d after reload, want -1 (stale selection must not survive)", m.selectedCust)
	}
}

// ---- Row interaction: click selects, double-click edits -------------------

// seedPricedInvoice saves an invoice carrying one line item, so the row and
// the footer summary have real figures to render.
func seedPricedInvoice(t *testing.T, db *store.DB, number string, typ model.InvoiceType, currency string, rate money.Amount) {
	t.Helper()
	if _, err := db.SaveInvoice(model.Invoice{
		Type:     typ,
		Number:   number,
		Date:     time.Now(),
		Customer: model.Customer{Name: "Acme Co"},
		Currency: currency,
		FXFactor: money.Rate(80 * money.RateScale),
		Items: []model.LineItem{{
			Description: "Services",
			Quantity:    money.Qty(100), // 1.00, scaled to two decimals
			RateUSD:     rate,
			TaxRatePct:  1800,
		}},
	}); err != nil {
		t.Fatalf("SaveInvoice(%q): %v", number, err)
	}
}

// seedTaxedInvoice saves an invoice with a real seller state and place of
// supply, so calc.TaxModeFor has enough to work with to produce a genuine
// CGST+SGST or IGST split (see calc.domesticSameState) rather than the
// zero-everything result a blank Seller/PlaceOfSupply would silently fall
// back to. Returns the built invoice (unsaved-ID copy) so callers can run it
// through calc.ComputeInvoice themselves to derive expected figures instead
// of hand-computing GST maths in the test.
func seedTaxedInvoice(t *testing.T, db *store.DB, number string, typ model.InvoiceType, placeOfSupply string, rateUSD money.Amount) model.Invoice {
	t.Helper()
	currency, fx := "USD", money.Rate(80*money.RateScale)
	if typ == model.InvoiceDomestic {
		// CurrencyLocked in the editor pins domestic invoices to INR at a
		// factor of 1; mirrored here so the fixture matches what the app
		// would ever actually save.
		currency, fx = "INR", money.Rate(1*money.RateScale)
	}
	inv := model.Invoice{
		Type:          typ,
		Number:        number,
		Date:          time.Now(),
		Customer:      model.Customer{Name: "Acme Co"},
		PlaceOfSupply: placeOfSupply,
		Currency:      currency,
		FXFactor:      fx,
		Seller:        model.Company{StateCode: "33", StateName: "Tamil Nadu"},
		Items: []model.LineItem{{
			Description: "Services",
			Quantity:    money.Qty(100), // 1.00, scaled to two decimals
			RateUSD:     rateUSD,
			TaxRatePct:  1800,
		}},
	}
	if _, err := db.SaveInvoice(inv); err != nil {
		t.Fatalf("SaveInvoice(%q): %v", number, err)
	}
	return inv
}

// ---- CGST/SGST/IGST columns ------------------------------------------------

// TestInvoiceRow_DomesticIntraStateShowsCGSTAndSGST covers the case
// calc.TaxModeFor calls TaxCGSTSGST: a domestic invoice whose recipient's
// place of supply matches the seller's own state (both "33", Tamil Nadu)
// splits its tax into CGST+SGST rather than charging a single IGST. The row
// must show real figures under CGST and SGST and the "not applicable"
// em-dash — not a printed zero — under the IGST column this invoice never
// charges.
func TestInvoiceRow_DomesticIntraStateShowsCGSTAndSGST(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	inv := seedTaxedInvoice(t, db, "DOM-1", model.InvoiceDomestic, "33-Tamil Nadu", money.Amount(1000000))
	result := calc.ComputeInvoice(&inv)
	if result.CGST == 0 || result.SGST == 0 || result.IGST != 0 {
		t.Fatalf("fixture is not intra-state domestic (CGST=%d SGST=%d IGST=%d); check seller/place-of-supply state codes", result.CGST, result.SGST, result.IGST)
	}
	s.Reload()

	row := newInvoiceRow(func(widget.ListItemID) {}, func(widget.ListItemID) {})
	row.set(0, s.invoices[0])

	if got, want := row.cells[colCGST].Text, "₹ "+money.FormatINR(result.CGST); got != want {
		t.Errorf("CGST cell = %q, want %q", got, want)
	}
	if got, want := row.cells[colSGST].Text, "₹ "+money.FormatINR(result.SGST); got != want {
		t.Errorf("SGST cell = %q, want %q", got, want)
	}
	if got, want := row.cells[colIGST].Text, "—"; got != want {
		t.Errorf("IGST cell = %q, want %q (intra-state domestic never charges IGST)", got, want)
	}
	if got, want := row.cells[colIGST].Color, themeColor(theme.ColorNamePlaceHolder); got != want {
		t.Errorf("IGST cell color = %v, want the placeholder tone the USD Total column already uses for a not-applicable figure", got)
	}
}

// TestInvoiceRow_ExportIGSTShowsRealIGSTFigure is the regression test for the
// wrong-looking shortcut this feature's brief explicitly warns against:
// gating the whole tax split on "is this invoice domestic" would zero out
// IGST here, even though model.InvoiceExportIGST genuinely charges it (see
// calc.ComputeLine's export_igst case). Blanking a tax that was actually
// charged would misreport it, not just omit a column that doesn't apply.
func TestInvoiceRow_ExportIGSTShowsRealIGSTFigure(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	inv := seedTaxedInvoice(t, db, "AEXI-1", model.InvoiceExportIGST, "97-Other Territory", money.Amount(100000))
	result := calc.ComputeInvoice(&inv)
	if result.IGST == 0 || result.CGST != 0 || result.SGST != 0 {
		t.Fatalf("fixture does not charge IGST alone (CGST=%d SGST=%d IGST=%d)", result.CGST, result.SGST, result.IGST)
	}
	s.Reload()

	row := newInvoiceRow(func(widget.ListItemID) {}, func(widget.ListItemID) {})
	row.set(0, s.invoices[0])

	if got, want := row.cells[colIGST].Text, "₹ "+money.FormatINR(result.IGST); got != want {
		t.Errorf("IGST cell = %q, want %q", got, want)
	}
	for _, col := range []int{colCGST, colSGST} {
		if got, want := row.cells[col].Text, "—"; got != want {
			t.Errorf("column %d = %q, want %q (export_igst charges no CGST/SGST)", col, got, want)
		}
	}
}

// TestInvoiceRow_ExportLUTShowsDashesForAllThreeTaxColumns covers the third
// leg of calc.ComputeLine's switch: zero-rated-under-LUT exports charge none
// of the three taxes, so every one of the new columns should read the
// not-applicable em-dash rather than "₹ 0.00".
func TestInvoiceRow_ExportLUTShowsDashesForAllThreeTaxColumns(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedTaxedInvoice(t, db, "AEX-1", model.InvoiceExportLUT, "97-Other Territory", money.Amount(100000))
	s.Reload()

	row := newInvoiceRow(func(widget.ListItemID) {}, func(widget.ListItemID) {})
	row.set(0, s.invoices[0])

	for _, col := range []int{colCGST, colSGST, colIGST} {
		if got, want := row.cells[col].Text, "—"; got != want {
			t.Errorf("column %d = %q, want %q (zero-rated under LUT)", col, got, want)
		}
	}
}

// TestInvoicesScreen_SortByIGSTDefaultsDescending checks the new columns went
// into both switches a click needs: applySort's comparator and
// defaultSortAscending's "biggest first" default, the same as the existing
// INR/USD Total columns.
func TestInvoicesScreen_SortByIGSTDefaultsDescending(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedTaxedInvoice(t, db, "AEXI-1", model.InvoiceExportIGST, "97-Other Territory", money.Amount(100000))
	seedTaxedInvoice(t, db, "AEXI-3", model.InvoiceExportIGST, "97-Other Territory", money.Amount(300000))
	seedTaxedInvoice(t, db, "AEXI-2", model.InvoiceExportIGST, "97-Other Territory", money.Amount(200000))
	s.Reload()

	s.handleHeaderClick(colIGST)
	if s.sortCol != colIGST || s.sortAsc {
		t.Fatalf("after first click on IGST: sortCol=%d sortAsc=%v, want col=%d asc=false", s.sortCol, s.sortAsc, colIGST)
	}
	if got, want := numbersOf(s.invoices), []string{"AEXI-3", "AEXI-2", "AEXI-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("descending sort by IGST = %v, want %v", got, want)
	}

	s.handleHeaderClick(colIGST)
	if got, want := numbersOf(s.invoices), []string{"AEXI-1", "AEXI-2", "AEXI-3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ascending sort by IGST (second click) = %v, want %v", got, want)
	}
}

// TestInvoicesScreen_FooterTaxTotals checks the footer sums CGST/SGST/IGST
// over the filtered set independently — exactly the figures a GSTR-1 return
// for a filtered month or FY would need — rather than only ever totalling
// INR/USD as before this feature.
func TestInvoicesScreen_FooterTaxTotals(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	dom := seedTaxedInvoice(t, db, "DOM-1", model.InvoiceDomestic, "33-Tamil Nadu", money.Amount(1000000))
	igst := seedTaxedInvoice(t, db, "AEXI-1", model.InvoiceExportIGST, "97-Other Territory", money.Amount(500000))
	domResult := calc.ComputeInvoice(&dom)
	igstResult := calc.ComputeInvoice(&igst)
	s.Reload()

	if got, want := s.footerCGST.Text, "₹ "+money.FormatINR(domResult.CGST); got != want {
		t.Errorf("footer CGST = %q, want %q", got, want)
	}
	if got, want := s.footerSGST.Text, "₹ "+money.FormatINR(domResult.SGST); got != want {
		t.Errorf("footer SGST = %q, want %q", got, want)
	}
	if got, want := s.footerIGST.Text, "₹ "+money.FormatINR(igstResult.IGST); got != want {
		t.Errorf("footer IGST = %q, want %q", got, want)
	}
}

// TestInvoicesScreen_FooterTaxTotalsBlankWhenNoneCharged mirrors the existing
// footerUSD "no dollar-billed invoices" case: with only a zero-rated LUT
// export in the filtered set, none of the three taxes are ever charged, so
// all three totals should read blank rather than a "₹ 0.00" that would look
// like a genuine, if quiet, month.
func TestInvoicesScreen_FooterTaxTotalsBlankWhenNoneCharged(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedTaxedInvoice(t, db, "AEX-1", model.InvoiceExportLUT, "97-Other Territory", money.Amount(100000))
	s.Reload()

	for _, c := range []*canvas.Text{s.footerCGST, s.footerSGST, s.footerIGST} {
		if c.Text != "" {
			t.Errorf("footer tax total = %q, want blank when nothing in the filtered set charges it", c.Text)
		}
	}
}

// TestInvoiceRow_ClickSelectsDoubleClickEdits covers the row's own pointer
// handling. Selection deliberately happens on mouse-down rather than on a
// tap: Fyne holds a tap back for the length of the double-tap interval, so
// selecting on tap would leave the toolbar a beat behind the pointer.
func TestInvoiceRow_ClickSelectsDoubleClickEdits(t *testing.T) {
	s, db := newTestInvoicesScreen(t)

	var opened []string
	s.onOpenEditor = func(st *EditorState) { opened = append(opened, st.Number) }

	seedInvoice(t, db, "AEX-1", "Acme Co", time.Now())
	s.Reload()

	row := newInvoiceRow(func(id widget.ListItemID) { s.list.Select(id) }, s.editRow)
	row.set(0, s.invoices[0])

	row.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	if s.selected != 0 {
		t.Fatalf("selected = %d after click, want 0", s.selected)
	}
	if len(opened) != 0 {
		t.Fatalf("a single click opened the editor (%v); it must only select", opened)
	}

	row.DoubleTapped(&fyne.PointEvent{})
	if want := []string{"AEX-1"}; !reflect.DeepEqual(opened, want) {
		t.Errorf("opened = %v after double-click, want %v", opened, want)
	}
}

// TestInvoiceRow_SecondaryClickDoesNotSelect: a right-click that moved the
// selection would arm the toolbar's Delete against an invoice the user never
// chose.
func TestInvoiceRow_SecondaryClickDoesNotSelect(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-1", "Acme Co", time.Now())
	s.Reload()

	row := newInvoiceRow(func(id widget.ListItemID) { s.list.Select(id) }, s.editRow)
	row.set(0, s.invoices[0])
	row.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonSecondary})

	if s.selected != -1 {
		t.Errorf("selected = %d after a secondary click, want -1 (unchanged)", s.selected)
	}
}

// TestInvoiceRow_EditRowIgnoresStaleIndex: rows are recycled and hold the
// index they were last filled with, so a double-click arriving after the
// list shrank must not index past the slice.
func TestInvoiceRow_EditRowIgnoresStaleIndex(t *testing.T) {
	s, _ := newTestInvoicesScreen(t)
	opened := 0
	s.onOpenEditor = func(*EditorState) { opened++ }

	s.editRow(7) // nothing loaded at all
	if opened != 0 {
		t.Errorf("editRow(7) on an empty list opened %d editors, want 0", opened)
	}
}

// ---- Footer summary ------------------------------------------------------

// TestInvoicesScreen_SummaryTotals: the footer totals every invoice in INR
// (every invoice is raised in rupees) but only the dollar-billed ones in USD.
func TestInvoicesScreen_SummaryTotals(t *testing.T) {
	s, db := newTestInvoicesScreen(t)

	seedPricedInvoice(t, db, "AEX-1", model.InvoiceExportLUT, "USD", money.Amount(100000)) // $1,000.00
	seedPricedInvoice(t, db, "DOM-1", model.InvoiceDomestic, "INR", money.Amount(50000))   // no USD leg
	s.Reload()

	if got, want := s.footerCount.Text, "2 invoices"; got != want {
		t.Errorf("footer count = %q, want %q", got, want)
	}

	// $1,000.00 at a factor of 80 is ₹80,000.00, carrying no GST under an
	// LUT; the domestic invoice's ₹40,000.00 taxable value plus 18% is
	// ₹47,200.00.
	if got, want := s.footerINR.Text, "₹ 1,27,200.00"; got != want {
		t.Errorf("footer INR = %q, want %q", got, want)
	}
	// The domestic invoice has a non-zero TotalUSD of its own — the rupee
	// figures are derived from a dollar rate — so the currency filter, not
	// the amount, is what has to keep it out of the dollar total.
	if got, want := s.footerUSD.Text, "$ 1,000.00"; got != want {
		t.Errorf("footer USD = %q, want %q", got, want)
	}
}

// TestInvoicesScreen_SummaryHiddenWhenEmpty: with no invoices the empty state
// speaks for the sheet, and a "0 invoices / ₹ 0.00" band under it would read
// as a real total rather than as an absence.
func TestInvoicesScreen_SummaryHiddenWhenEmpty(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	if s.footer.Visible() {
		t.Error("footer is visible with no invoices, want hidden")
	}

	seedInvoice(t, db, "AEX-1", "Acme Co", time.Now())
	s.Reload()
	if !s.footer.Visible() {
		t.Error("footer is hidden with an invoice loaded, want visible")
	}

	// An invoice billed only in rupees leaves the USD cell blank rather than
	// printing a "$ 0.00" that would read as a figure someone was charged.
	seedPricedInvoice(t, db, "DOM-1", model.InvoiceDomestic, "INR", money.Amount(50000))
	if err := db.DeleteInvoice(s.invoices[0].ID); err != nil {
		t.Fatalf("DeleteInvoice: %v", err)
	}
	s.Reload()
	if s.footerUSD.Text != "" {
		t.Errorf("footer USD = %q with no dollar-billed invoice, want empty", s.footerUSD.Text)
	}
}

// ---- Filter bar ------------------------------------------------------

// TestInvoicesScreen_SearchFiltersFooterShowsNOfM covers the search box end
// to end: typing into s.searchEntry (via SetText, which fires OnChanged the
// same way a keystroke would) should narrow s.invoices and switch the
// footer count from "N invoices" to "shown of total invoices".
func TestInvoicesScreen_SearchFiltersFooterShowsNOfM(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-2", "Beta LLC", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-3", "Acme Corp", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	s.Reload()

	if got, want := s.footerCount.Text, "3 invoices"; got != want {
		t.Fatalf("footer count before filtering = %q, want %q", got, want)
	}

	s.searchEntry.SetText("Acme")
	if got, want := len(s.invoices), 2; got != want {
		t.Fatalf("filtered invoice count = %d, want %d", got, want)
	}
	if got, want := s.footerCount.Text, "2 of 3 invoices"; got != want {
		t.Errorf("footer count while filtered = %q, want %q", got, want)
	}

	s.searchEntry.SetText("")
	if got, want := s.footerCount.Text, "3 invoices"; got != want {
		t.Errorf("footer count after clearing the search box = %q, want %q", got, want)
	}
}

// TestInvoicesScreen_FooterSingularTotal covers the singular/plural edge of
// invoiceCountLabel's filtered form: with only one invoice in the whole
// database, "of 1" must read "invoice", not "invoices".
func TestInvoicesScreen_FooterSingularTotal(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	s.Reload()

	s.searchEntry.SetText("Acme")
	if got, want := s.footerCount.Text, "1 of 1 invoice"; got != want {
		t.Errorf("footer count = %q, want %q", got, want)
	}

	s.searchEntry.SetText("nobody")
	if got, want := s.footerCount.Text, "0 of 1 invoice"; got != want {
		t.Errorf("footer count with zero matches = %q, want %q", got, want)
	}
}

// TestInvoicesScreen_FilterChangeClearsSelection is the filter-bar analogue
// of TestInvoicesScreen_HeaderClickClearsSelection: narrowing the filter
// removes or reorders rows out from under whatever index was selected, so a
// stale s.selected must not survive it — the exact bug class the Reload and
// sort tests already guard against, here reached through a different door.
func TestInvoicesScreen_FilterChangeClearsSelection(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX-2", "Beta LLC", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	s.Reload()

	s.list.OnSelected(0)
	if s.selected != 0 {
		t.Fatalf("selected = %d, want 0", s.selected)
	}

	s.searchEntry.SetText("Acme")
	if s.selected != -1 {
		t.Errorf("selected = %d after the search box changed the row set, want -1", s.selected)
	}
}

// TestInvoicesScreen_ClearButtonTracksFilterState checks that Clear stays
// disabled with no filter active, lights up once one is, and resets every
// control (including the FY/month selects) back to their defaults.
func TestInvoicesScreen_ClearButtonTracksFilterState(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC))
	s.Reload()

	if !s.clearBtn.Disabled() {
		t.Fatal("Clear should start disabled with no filter active")
	}

	s.searchEntry.SetText("Acme")
	if s.clearBtn.Disabled() {
		t.Error("Clear should enable once a filter is active")
	}

	s.clearFilters()
	if !s.clearBtn.Disabled() {
		t.Error("Clear should disable again once filters are cleared")
	}
	if s.searchEntry.Text != "" {
		t.Errorf("search entry text = %q after clearFilters, want empty", s.searchEntry.Text)
	}
	if s.fySelect.Selected != allPeriodsLabel {
		t.Errorf("FY select = %q after clearFilters, want %q", s.fySelect.Selected, allPeriodsLabel)
	}
	if s.monthSelect.Selected != allMonthsLabel {
		t.Errorf("month select = %q after clearFilters, want %q", s.monthSelect.Selected, allMonthsLabel)
	}
	if got, want := len(s.invoices), 1; got != want {
		t.Errorf("invoices shown after clearFilters = %d, want %d (unfiltered)", got, want)
	}
}

// TestInvoicesScreen_FYFilterFallsBackWhenSelectedFYVanishes is the
// regression case for the edge Reload has to handle: the FY a user has
// filtered to can disappear (its last invoice deleted) between one Reload
// and the next. The dropdown must fall back to "All periods" rather than
// keep pointing at a period that would now hide every row.
func TestInvoicesScreen_FYFilterFallsBackWhenSelectedFYVanishes(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	// 15 May 2024 falls in FY 2024-25; 15 May 2025 falls in FY 2025-26.
	id2425 := seedInvoice(t, db, "AEX2425-1", "Acme Corp", time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "AEX2526-1", "Beta LLC", time.Date(2025, 5, 15, 0, 0, 0, 0, time.UTC))
	s.Reload()

	s.fySelect.Selected = fyLabel("2425")
	s.filter.fy = "2425"
	s.applyFilter()
	if got, want := len(s.invoices), 1; got != want {
		t.Fatalf("invoices shown for FY 2024-25 = %d, want %d", got, want)
	}

	// Delete the only invoice in FY 2024-25, so that FY no longer appears
	// anywhere in the data, and reload as if the delete flow had just run.
	if err := db.DeleteInvoice(id2425); err != nil {
		t.Fatalf("DeleteInvoice: %v", err)
	}
	s.Reload()

	if s.filter.fy != "" {
		t.Errorf("filter.fy = %q after its FY vanished, want \"\" (All periods)", s.filter.fy)
	}
	if s.fySelect.Selected != allPeriodsLabel {
		t.Errorf("FY select = %q after its FY vanished, want %q", s.fySelect.Selected, allPeriodsLabel)
	}
	if got, want := len(s.invoices), 1; got != want {
		t.Errorf("invoices shown after fallback = %d, want %d (the one remaining invoice)", got, want)
	}
}

// TestInvoicesScreen_NoMatchesEmptyStateDistinctFromNoData checks the two
// zero-row conditions are told apart: an empty database shows s.empty, while
// a filter that excludes every real invoice shows s.noMatches instead, with
// the footer still visible in the latter case (see syncEmptyState).
func TestInvoicesScreen_NoMatchesEmptyStateDistinctFromNoData(t *testing.T) {
	s, db := newTestInvoicesScreen(t)

	if !s.empty.Visible() {
		t.Error("empty should be visible with no invoices at all")
	}
	if s.noMatches.Visible() {
		t.Error("noMatches should not be visible with no invoices at all")
	}

	seedInvoice(t, db, "AEX-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	s.Reload()
	s.searchEntry.SetText("nobody by this name")

	if s.empty.Visible() {
		t.Error("empty should not be visible once there are real invoices")
	}
	if !s.noMatches.Visible() {
		t.Error("noMatches should be visible once a filter excludes every invoice")
	}
	if !s.footer.Visible() {
		t.Error("footer should stay visible for a filter matching nothing (it reports 0 of N)")
	}
	if s.pager.Visible() {
		t.Error("pager should hide when the filter matches nothing")
	}
}

// ---- Pagination ----------------------------------------------------------

// withPageSize lowers pageSize so multi-page scenarios don't need a full
// invoicePageSize of fixture rows.
func withPageSize(s *InvoicesScreen, pageSize int) {
	if pageSize > 0 {
		s.pageSize = pageSize
	}
}

// TestInvoicesScreen_PaginationOnByDefault is the contract that paging is
// always active: a multi-row history is sliced to pageSize and the pager
// appears once the set spills past one page.
func TestInvoicesScreen_PaginationOnByDefault(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	withPageSize(s, 2)
	seedInvoice(t, db, "A-1", "Alice", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "B-1", "Bob", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "C-1", "Carol", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	s.Reload()

	if got, want := s.list.Length(), 2; got != want {
		t.Errorf("list length = %d, want %d (one page only)", got, want)
	}
	if !s.pager.Visible() {
		t.Error("pager should show when the filtered set spans more than one page")
	}
}

// TestInvoicesScreen_PaginationPagesListKeepsFullFooterTotals checks the
// core contract: the list only shows one pageSize of rows, while the
// footer still totals every filtered invoice (page-independent period sums).
func TestInvoicesScreen_PaginationPagesListKeepsFullFooterTotals(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	withPageSize(s, 2)
	// Three priced invoices so footer INR/USD have real multi-row sums that
	// must not shrink when the list is showing a single page of two.
	seedPricedInvoice(t, db, "A-1", model.InvoiceExportLUT, "USD", money.Amount(100000)) // $1,000
	seedPricedInvoice(t, db, "A-2", model.InvoiceExportLUT, "USD", money.Amount(200000)) // $2,000
	seedPricedInvoice(t, db, "A-3", model.InvoiceExportLUT, "USD", money.Amount(300000)) // $3,000
	s.Reload()

	if got, want := len(s.invoices), 3; got != want {
		t.Fatalf("filtered set = %d, want %d", got, want)
	}
	if got, want := s.pageLength(), 2; got != want {
		t.Fatalf("page length = %d, want %d", got, want)
	}
	if got, want := s.list.Length(), 2; got != want {
		t.Fatalf("list length = %d, want %d (one page only)", got, want)
	}
	if !s.pager.Visible() {
		t.Fatal("pager should show when the filtered set spans more than one page")
	}
	if got, want := s.pageLabel.Text, "Page 1 of 2 · 1–2"; got != want {
		t.Errorf("page label = %q, want %q", got, want)
	}
	// $6,000 at FX 80 is ₹4,80,000 with no GST under LUT.
	if got, want := s.footerINR.Text, "₹ 4,80,000.00"; got != want {
		t.Errorf("footer INR = %q, want %q (full filtered set, not just the page)", got, want)
	}
	if got, want := s.footerUSD.Text, "$ 6,000.00"; got != want {
		t.Errorf("footer USD = %q, want %q (full filtered set, not just the page)", got, want)
	}
	if got, want := s.footerCount.Text, "3 invoices"; got != want {
		t.Errorf("footer count = %q, want %q", got, want)
	}
}

// TestInvoicesScreen_PaginationNextPrevNavigatesAndClearsSelection covers
// page turns, selection safety across a turn, and the final short page.
func TestInvoicesScreen_PaginationNextPrevNavigatesAndClearsSelection(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	withPageSize(s, 2)
	seedInvoice(t, db, "A-1", "Alice", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "B-1", "Bob", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "C-1", "Carol", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "D-1", "Dave", time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "E-1", "Eve", time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC))
	s.Reload()

	// Store order is most-recently-created first: E, D, C, B, A.
	if got, want := numbersOf(s.invoices[:2]), []string{"E-1", "D-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first page of filtered set = %v, want %v", got, want)
	}

	s.list.OnSelected(0)
	if inv := s.selectedInvoice(); inv == nil || inv.Number != "E-1" {
		t.Fatalf("selectedInvoice = %v, want E-1", inv)
	}

	s.changePage(1)
	if s.page != 1 {
		t.Fatalf("page after Next = %d, want 1", s.page)
	}
	if s.selected != -1 {
		t.Errorf("selected = %d after page change, want -1", s.selected)
	}
	if s.selectedInvoice() != nil {
		t.Error("selectedInvoice should be nil after a page change")
	}
	if got, want := s.pageLength(), 2; got != want {
		t.Fatalf("page 2 length = %d, want %d", got, want)
	}
	// List item 0 on page 2 is C-1 (filtered index 2).
	s.list.OnSelected(0)
	if inv := s.selectedInvoice(); inv == nil || inv.Number != "C-1" {
		t.Fatalf("selectedInvoice on page 2 = %v, want C-1", inv)
	}

	s.changePage(1) // last page: E/D | C/B | A  → one row
	if got, want := s.pageLength(), 1; got != want {
		t.Fatalf("last page length = %d, want %d", got, want)
	}
	if got, want := s.pageLabel.Text, "Page 3 of 3 · 5–5"; got != want {
		t.Errorf("last page label = %q, want %q", got, want)
	}
	if !s.nextPageBtn.Disabled() {
		t.Error("Next should disable on the last page")
	}
	if s.prevPageBtn.Disabled() {
		t.Error("Previous should stay enabled on the last page")
	}

	s.changePage(1) // past the end: no-op
	if s.page != 2 {
		t.Errorf("page after Next past last = %d, want 2", s.page)
	}

	s.changePage(-1)
	if s.page != 1 {
		t.Errorf("page after Previous = %d, want 1", s.page)
	}
}

// TestInvoicesScreen_FilterResetsToFirstPage ensures a filter that shrinks
// the set never leaves the user stranded on a now-empty later page.
func TestInvoicesScreen_FilterResetsToFirstPage(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	withPageSize(s, 2)
	seedInvoice(t, db, "A-1", "Acme Corp", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "B-1", "Beta LLC", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "C-1", "Acme Corp", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "D-1", "Delta Inc", time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "E-1", "Acme Corp", time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC))
	s.Reload()

	s.changePage(2)
	if s.page != 2 {
		t.Fatalf("page = %d, want 2 before filtering", s.page)
	}

	s.searchEntry.SetText("Acme")
	if s.page != 0 {
		t.Errorf("page = %d after filter, want 0 (restart at the top of the result set)", s.page)
	}
	// Three Acme invoices → two pages of pageSize 2; pager stays visible.
	if got, want := len(s.invoices), 3; got != want {
		t.Fatalf("filtered count = %d, want %d", got, want)
	}
	if !s.pager.Visible() {
		t.Error("pager should remain visible with 3 matches and pageSize 2")
	}
}

// TestInvoicesScreen_PagerHiddenWhenSinglePage: short histories should not
// grow permanently-disabled Prev/Next chrome.
func TestInvoicesScreen_PagerHiddenWhenSinglePage(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	// Leave pageSize at the package default — two rows fit on one page.
	seedInvoice(t, db, "A-1", "Acme", time.Now())
	seedInvoice(t, db, "B-1", "Beta", time.Now())
	s.Reload()

	if s.pager.Visible() {
		t.Error("pager should hide when the full set fits on one page")
	}
	if got, want := s.pageLength(), 2; got != want {
		t.Errorf("page length = %d, want %d", got, want)
	}
}

// TestInvoicesScreen_EditRowRespectsPageOffset: double-clicking list item 0
// on page 2 must open the filtered invoice at that page's offset, not
// invoices[0].
func TestInvoicesScreen_EditRowRespectsPageOffset(t *testing.T) {
	s, db := newTestInvoicesScreen(t)
	withPageSize(s, 2)
	var opened []string
	s.onOpenEditor = func(st *EditorState) { opened = append(opened, st.Number) }

	seedInvoice(t, db, "A-1", "Alice", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "B-1", "Bob", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "C-1", "Carol", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
	seedInvoice(t, db, "D-1", "Dave", time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC))
	s.Reload()
	// Store order is most-recently-created first: D, C, B, A.
	// Page 0: D, C. Page 1: B, A. editRow(0) on page 1 must open B, not D.
	s.changePage(1)
	s.editRow(0)

	if got, want := opened, []string{"B-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("opened = %v, want %v (list index 0 on page 2 is B-1, not D-1)", got, want)
	}
}
