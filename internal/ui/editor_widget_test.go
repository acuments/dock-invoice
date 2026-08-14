package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/model"
)

// newTestEditor builds an Editor inside a headless test window, per
// fyne.io/fyne/v2/test, so these tests exercise the real widgets (not just
// the pure EditorState logic covered in editorstate_test.go).
func newTestEditor(t *testing.T, state *EditorState) (*Editor, func(model.Invoice) error) {
	t.Helper()
	test.NewTempApp(t)
	var saved *model.Invoice
	saveFn := func(inv model.Invoice) error {
		saved = &inv
		return nil
	}
	ed := NewEditor(state, model.Settings{}, EditorMasters{}, saveFn, func() {})
	win := test.NewTempWindow(t, ed.Container())
	win.Resize(ed.Container().MinSize())
	_ = saved
	return ed, saveFn
}

// newTestEditorWithMasters is newTestEditor with non-empty Masters lists
// so picker-driven flows can be exercised headlessly.
func newTestEditorWithMasters(t *testing.T, state *EditorState, masters EditorMasters) *Editor {
	t.Helper()
	test.NewTempApp(t)
	ed := NewEditor(state, model.Settings{Defaults: model.Defaults{TaxRatePct: 1800}}, masters, func(model.Invoice) error { return nil }, func() {})
	win := test.NewTempWindow(t, ed.Container())
	win.Resize(ed.Container().MinSize())
	return ed
}

func TestEditorWidget_TypeSwitch_TogglesShippingFieldsVisibility(t *testing.T) {
	state := goldenState() // export_lut: shipping fields shown
	ed, _ := newTestEditor(t, state)

	if !ed.shippingBox.Visible() {
		t.Fatal("shipping box should start visible for export_lut")
	}

	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceDomestic))
	if ed.shippingBox.Visible() {
		t.Error("shipping box should be hidden after switching to domestic")
	}
	if !ed.currencyEntry.Disabled() {
		t.Error("currency entry should be disabled for domestic")
	}
	if !ed.fxEntry.Disabled() {
		t.Error("FX factor entry should be disabled for domestic")
	}
	if ed.currencyEntry.Text != "INR" {
		t.Errorf("currency entry text = %q, want INR", ed.currencyEntry.Text)
	}

	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceExportIGST))
	if !ed.shippingBox.Visible() {
		t.Error("shipping box should be visible again for export_igst")
	}
	if ed.currencyEntry.Disabled() {
		t.Error("currency entry should be enabled again for export_igst")
	}
	if ed.shippingReq.Text == "" {
		t.Error("export_igst should show a shipping-required hint")
	}
}

// TestEditorWidget_TypeSwitch_SyncsWidgetTextWithState is a regression test
// for B1: applyTypeVisibility used to only Enable()/Hide() widgets on a type
// change, leaving stale text in the currency/FX/shipping-bill entries even
// though EditorState.SetType had already moved the underlying state on. A
// state-only test can't catch this — the state was always correct, only the
// on-screen widget text lagged behind — so this asserts the actual displayed
// Entry.Text after each switch.
func TestEditorWidget_TypeSwitch_SyncsWidgetTextWithState(t *testing.T) {
	state := goldenState() // export_lut: Currency "USD", FXFactor "83.2"
	ed, _ := newTestEditor(t, state)

	if ed.currencyEntry.Text != "USD" {
		t.Fatalf("currency entry text = %q, want USD before any switch", ed.currencyEntry.Text)
	}

	// export_lut -> domestic: currency/FX lock to INR/1, and both the
	// widgets and the state must show it.
	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceDomestic))
	if ed.currencyEntry.Text != "INR" {
		t.Errorf("currency entry text = %q, want INR after switching to domestic", ed.currencyEntry.Text)
	}
	if ed.fxEntry.Text != "1" {
		t.Errorf("FX entry text = %q, want 1 after switching to domestic", ed.fxEntry.Text)
	}

	// domestic -> export_lut: SetType restores state.Currency to "USD"; the
	// widget must follow, not keep showing "INR".
	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceExportLUT))
	if ed.currencyEntry.Text != "USD" {
		t.Errorf("currency entry text = %q, want USD after switching back to export_lut (was showing stale INR before the B1 fix)", ed.currencyEntry.Text)
	}
	if ed.state.Currency != ed.currencyEntry.Text {
		t.Errorf("currency entry text %q disagrees with state.Currency %q", ed.currencyEntry.Text, ed.state.Currency)
	}
}

// TestEditorWidget_TypeSwitch_ClearsShippingWidgetsOnDomestic is a regression
// test for the second half of B1: switching an export type to domestic
// blanks ShippingBillNo/Date/PortCode in state, but the widgets were only
// hidden, never cleared — so switching back to an export type resurrected
// the old text even though state.Validate() would (correctly) still treat
// the fields as empty, with nothing visible explaining why Save failed.
func TestEditorWidget_TypeSwitch_ClearsShippingWidgetsOnDomestic(t *testing.T) {
	state := goldenState() // export_lut
	ed, _ := newTestEditor(t, state)

	test.Type(ed.sbNoEntry, "SB1")
	sbDate := time.Date(2024, 5, 1, 0, 0, 0, 0, time.Local)
	ed.sbDateEntry.SetDate(&sbDate)
	// SetDate updates the widget; also push the ISO form state keeps for Build.
	ed.state.ShippingBillDate = "2024-05-01"
	test.Type(ed.sbPortEntry, "INMAA1")
	if ed.state.ShippingBillNo != "SB1" {
		t.Fatalf("setup: expected ShippingBillNo to be typed into state, got %q", ed.state.ShippingBillNo)
	}

	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceDomestic))
	if ed.sbNoEntry.Text != "" || ed.sbDateEntry.Text != "" || ed.sbPortEntry.Text != "" {
		t.Errorf("shipping entries should be cleared on switch to domestic, got %q / %q / %q",
			ed.sbNoEntry.Text, ed.sbDateEntry.Text, ed.sbPortEntry.Text)
	}

	// Switch back to an export type: the widgets must stay empty (matching
	// the now-empty state), not resurrect the old "SB1" / etc. text.
	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceExportIGST))
	if ed.sbNoEntry.Text != "" || ed.sbDateEntry.Text != "" || ed.sbPortEntry.Text != "" {
		t.Errorf("shipping entries should stay empty after switching back to export_igst, got %q / %q / %q (state: %q / %q / %q)",
			ed.sbNoEntry.Text, ed.sbDateEntry.Text, ed.sbPortEntry.Text,
			ed.state.ShippingBillNo, ed.state.ShippingBillDate, ed.state.ShippingPortCode)
	}
}

func TestEditorWidget_Recalc_UpdatesTotalsOnKeystroke(t *testing.T) {
	state := goldenState()
	ed, _ := newTestEditor(t, state)

	if !strings.Contains(ed.totalsLabel.String(), "3,32,800.00") {
		t.Fatalf("expected initial totals to show 3,32,800.00, got: %s", ed.totalsLabel.String())
	}

	// Find the rate entry for the single seeded line item and retype it,
	// character by character, via the real Fyne key-event pipeline.
	// First card in itemsBox.
	row := ed.itemsBox.Objects[0]
	rateEntry := findRateEntry(t, row)
	rateEntry.SetText("")
	test.Type(rateEntry, "8000.00")

	if got := ed.totalsLabel.String(); !strings.Contains(got, "6,65,600.00") {
		t.Errorf("expected totals to update to 6,65,600.00 (8000 * 83.2) after keystrokes, got: %s", got)
	}
}

// TestEditorWidget_LineRow_ShowsPerLineAmount is a regression test for B2:
// each line-item row must show its own computed amount (from
// calc.LineResult.TotalINR, via EditorState.Recalc) and keep it updated on
// every keystroke through the single recalc() entry point — not just the
// invoice-level totals label.
func TestEditorWidget_LineRow_ShowsPerLineAmount(t *testing.T) {
	state := goldenState() // single line: qty 1.00 * rate 4000.00 USD @ 83.2 FX, LUT (zero tax)
	ed, _ := newTestEditor(t, state)

	if len(ed.lineAmountLabels) != 1 {
		t.Fatalf("expected 1 line amount label, got %d", len(ed.lineAmountLabels))
	}
	// 4000.00 * 83.2 = 332,800.00 INR taxable, zero tax under LUT.
	if got := ed.lineAmountLabels[0].Text; !strings.Contains(got, "3,32,800.00") {
		t.Errorf("line amount label = %q, want it to contain 3,32,800.00", got)
	}

	row := ed.itemsBox.Objects[0]
	rateEntry := findRateEntry(t, row)
	rateEntry.SetText("")
	test.Type(rateEntry, "8000.00")

	if got := ed.lineAmountLabels[0].Text; !strings.Contains(got, "6,65,600.00") {
		t.Errorf("line amount label after keystroke edit = %q, want it to contain 6,65,600.00 (8000 * 83.2)", got)
	}
}

func TestEditorWidget_AddAndDeleteLine(t *testing.T) {
	state := goldenState()
	ed, _ := newTestEditor(t, state)

	if got := len(ed.lineAmountLabels); got != 1 {
		t.Fatalf("expected 1 line card initially, got %d", got)
	}

	state.AddLine("", "", 0)
	ed.refreshItems()
	if got := len(ed.lineAmountLabels); got != 2 {
		t.Fatalf("expected 2 line cards after AddLine+refresh, got %d", got)
	}

	del := findDeleteButton(t, ed.itemsBox.Objects[0])
	test.Tap(del)
	if len(state.Items) != 1 {
		t.Errorf("expected 1 remaining item after delete, got %d", len(state.Items))
	}
	if got := len(ed.lineAmountLabels); got != 1 {
		t.Errorf("expected 1 remaining line card after delete, got %d", got)
	}
}

func TestEditorWidget_Save_BlockedByValidation(t *testing.T) {
	state := &EditorState{Type: model.InvoiceExportLUT, FXFactor: "1"} // no customer, no items
	var savedCalled bool
	test.NewTempApp(t)
	ed := NewEditor(state, model.Settings{}, EditorMasters{}, func(model.Invoice) error {
		savedCalled = true
		return nil
	}, func() {})
	win := test.NewTempWindow(t, ed.Container())
	win.Resize(ed.Container().MinSize())

	saveBtn := findButtonByText(t, ed.Container(), "Save & export PDF")
	test.Tap(saveBtn)

	if savedCalled {
		t.Error("save callback should not fire when validation fails")
	}
	if ed.errorLabel.Text == "" {
		t.Error("expected the error label to show validation errors")
	}
}

// TestEditorWidget_TypeSelect_UsesHumanLabels is the editor-side counterpart
// of TestInvoicesScreen_TableShowsHumanReadableType (C1): the type dropdown
// must offer "Export (LUT)" etc., never raw enum values like "export_lut".
func TestEditorWidget_TypeSelect_UsesHumanLabels(t *testing.T) {
	state := goldenState()
	ed, _ := newTestEditor(t, state)

	for _, raw := range []string{"export_lut", "export_igst", "domestic"} {
		for _, opt := range ed.typeSelect.Options {
			if opt == raw {
				t.Errorf("type select still offers raw enum value %q among options %v", raw, ed.typeSelect.Options)
			}
		}
	}
	for _, want := range InvoiceTypeLabels() {
		found := false
		for _, opt := range ed.typeSelect.Options {
			if opt == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("type select missing human label %q; options = %v", want, ed.typeSelect.Options)
		}
	}
	if got := ed.typeSelect.Selected; got != InvoiceTypeLabel(model.InvoiceExportLUT) {
		t.Errorf("initial type select = %q, want %q", got, InvoiceTypeLabel(model.InvoiceExportLUT))
	}
}

// TestEditorWidget_CustomerPicker_FillsFormAndState is the regression test
// for the "Masters tab is decorative" gap: picking a saved customer must
// snapshot its fields into both EditorState and the free-text widgets.
func TestEditorWidget_CustomerPicker_FillsFormAndState(t *testing.T) {
	cust := model.Customer{
		ID:              7,
		Name:            "Acme Corp",
		GSTIN:           "33AAAAA0000A1Z5",
		BillingAddress:  []string{"1 Main St", "Chennai"},
		ShippingAddress: []string{"Dock 4"},
	}
	state := goldenState()
	ed := newTestEditorWithMasters(t, state, EditorMasters{Customers: []model.Customer{cust}})

	ed.applyCustomerToForm(cust)

	if ed.state.Customer.Name != "Acme Corp" || ed.state.Customer.GSTIN != "33AAAAA0000A1Z5" {
		t.Errorf("state.Customer after apply = %+v, want Acme Corp with GSTIN", ed.state.Customer)
	}
	if ed.custNameEntry.Text != "Acme Corp" {
		t.Errorf("name entry = %q, want Acme Corp", ed.custNameEntry.Text)
	}
	if ed.custGSTINEntry.Text != "33AAAAA0000A1Z5" {
		t.Errorf("GSTIN entry = %q, want 33AAAAA0000A1Z5", ed.custGSTINEntry.Text)
	}
	if ed.billingEntry.Text != "1 Main St\nChennai" {
		t.Errorf("billing entry = %q, want two billing lines", ed.billingEntry.Text)
	}
	if ed.shippingEntry.Text != "Dock 4" {
		t.Errorf("shipping entry = %q, want Dock 4", ed.shippingEntry.Text)
	}
}

// TestEditorWidget_ItemPicker_AppendsLineFromMaster covers the saved-item
// half of making Masters reachable from the editor: picking a template
// appends a prefilled line (description/HSN/unit/rate) without wiping
// existing rows.
func TestEditorWidget_ItemPicker_AppendsLineFromMaster(t *testing.T) {
	item := model.Item{
		Description: "Consulting",
		HSNSAC:      "998314",
		Unit:        "HRS",
		DefaultRate: 15000, // 150.00
		Currency:    model.CurrencyUSD,
	}
	state := goldenState() // already has one blank-ish line from the fixture
	ed := newTestEditorWithMasters(t, state, EditorMasters{Items: []model.Item{item}})

	before := len(ed.state.Items)
	ed.state.AddLineFromItem(item, 1800)
	ed.refreshItems()
	ed.recalc()

	if got := len(ed.state.Items); got != before+1 {
		t.Fatalf("items after AddLineFromItem = %d, want %d", got, before+1)
	}
	li := ed.state.Items[len(ed.state.Items)-1]
	if li.Description != "Consulting" || li.HSNSAC != "998314" || li.Unit != "HRS" || li.RateUSD != "150.00" {
		t.Errorf("appended line = %+v, want Consulting / 998314 / HRS / 150.00", li)
	}
	if li.TaxRatePct != "18" {
		t.Errorf("appended line TaxRatePct = %q, want 18 (from settings defaults)", li.TaxRatePct)
	}
	if got, want := len(ed.lineAmountLabels), len(ed.state.Items); got != want {
		t.Errorf("line cards = %d, want %d", got, want)
	}
}

// TestCustomerComboOptions_KeepsDuplicateNamesDistinguishable is what the old
// label-disambiguation logic existed for, restated for the combobox: two
// customers sharing a name must stay independently pickable. Picking is by
// index now, so the labels themselves may collide — but the user still has to
// be able to tell the two rows apart, which is the GSTIN's job.
func TestCustomerComboOptions_KeepsDuplicateNamesDistinguishable(t *testing.T) {
	opts := customerComboOptions([]model.Customer{
		{Name: "Acme", GSTIN: "AAA"},
		{Name: "Acme", GSTIN: "BBB"},
		{Name: "", GSTIN: ""},
	})
	if len(opts) != 3 {
		t.Fatalf("opts = %v, want one per customer", opts)
	}
	if opts[0].Detail == opts[1].Detail {
		t.Errorf("the two Acme rows are indistinguishable: both read %q · %q", opts[0].Label, opts[0].Detail)
	}
	if opts[0].Detail != "AAA" || opts[1].Detail != "BBB" {
		t.Errorf("details = %q/%q, want the GSTINs", opts[0].Detail, opts[1].Detail)
	}
	if opts[2].Label != "(unnamed customer)" || opts[2].Detail != "no GSTIN" {
		t.Errorf("blank customer rendered as %q · %q, want readable placeholders", opts[2].Label, opts[2].Detail)
	}
}

// TestCustomerComboOptions_MatchesOnNameAndGSTIN pins the reason this is a
// search box rather than a Select: typing either half of the row finds it.
func TestCustomerComboOptions_MatchesOnNameAndGSTIN(t *testing.T) {
	opts := customerComboOptions([]model.Customer{
		{Name: "Acme Exports", GSTIN: "33AAAAA0000A1Z5"},
		{Name: "Beta Traders", GSTIN: "27BBBBB1111B2Z6"},
	})

	if got := matchComboOptions(opts, "beta", 40); len(got) != 1 || got[0] != 1 {
		t.Errorf("searching a name matched %v, want just Beta Traders", got)
	}
	if got := matchComboOptions(opts, "27BBBBB", 40); len(got) != 1 || got[0] != 1 {
		t.Errorf("searching a GSTIN matched %v, want just Beta Traders", got)
	}
	if got := matchComboOptions(opts, "", 40); len(got) != 2 {
		t.Errorf("empty query matched %v, want every customer", got)
	}
}

// TestSplitNonEmptyLines_DropsInteriorBlankLines is a regression test for
// B3: splitNonEmptyLines used to only trim a trailing blank line, leaving
// interior blank lines (e.g. from a pasted address, or the user hitting
// Enter twice) in the result despite what the function's name promises.
// Billing/shipping/company addresses are printed one line per slice entry,
// so a stray blank entry becomes a stray blank line on the invoice PDF.
func TestSplitNonEmptyLines_DropsInteriorBlankLines(t *testing.T) {
	got := splitNonEmptyLines("123 Main St\n\nSuite 400\nDallas, TX\n")
	want := []string{"123 Main St", "Suite 400", "Dallas, TX"}
	if len(got) != len(want) {
		t.Fatalf("splitNonEmptyLines = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitNonEmptyLines[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitNonEmptyLines_EmptyInputYieldsNoLines(t *testing.T) {
	if got := splitNonEmptyLines(""); len(got) != 0 {
		t.Errorf("splitNonEmptyLines(\"\") = %#v, want empty", got)
	}
	if got := splitNonEmptyLines("\n\n"); len(got) != 0 {
		t.Errorf("splitNonEmptyLines(\"\\n\\n\") = %#v, want empty", got)
	}
}

// ---- small widget-tree search helpers used only by these tests ----------

// walk visits root and every object beneath it, depth-first. Containers expose
// their children directly; widgets only expose theirs through a renderer, so
// those go through test.WidgetRenderer. The seen set guards against the cycles
// that renderers occasionally introduce.
func walk(root any, fn func(any)) {
	start, ok := root.(fyne.CanvasObject)
	if !ok {
		return
	}
	seen := make(map[fyne.CanvasObject]bool)
	var visit func(fyne.CanvasObject)
	visit = func(o fyne.CanvasObject) {
		if o == nil || seen[o] {
			return
		}
		seen[o] = true
		fn(o)
		switch typed := o.(type) {
		case *fyne.Container:
			for _, child := range typed.Objects {
				visit(child)
			}
		case *container.AppTabs:
			// Inactive tabs aren't always in the active renderer tree; still
			// walk every TabItem.Content so tests (and helpers) can find
			// widgets on Settings subsections without selecting each tab.
			for _, item := range typed.Items {
				if item != nil && item.Content != nil {
					visit(item.Content)
				}
			}
			for _, child := range test.WidgetRenderer(typed).Objects() {
				visit(child)
			}
		case fyne.Widget:
			for _, child := range test.WidgetRenderer(typed).Objects() {
				visit(child)
			}
		}
	}
	visit(start)
}

func findRateEntry(t *testing.T, root interface{}) *widget.Entry {
	t.Helper()
	var found *widget.Entry
	walk(root, func(o any) {
		if found != nil {
			return
		}
		if e, ok := o.(*widget.Entry); ok && e.Text == "4000.00" {
			found = e
		}
	})
	if found == nil {
		t.Fatal("could not find the rate entry (text 4000.00) in the line item card")
	}
	return found
}

func findDeleteButton(t *testing.T, root interface{}) *widget.Button {
	t.Helper()
	var found *widget.Button
	walk(root, func(o any) {
		if found != nil {
			return
		}
		if b, ok := o.(*widget.Button); ok && b.Text == "" {
			found = b
		}
	})
	if found == nil {
		t.Fatal("could not find the row's delete button")
	}
	return found
}

func findButtonByText(t *testing.T, root any, text string) *widget.Button {
	t.Helper()
	var found *widget.Button
	walk(root, func(o any) {
		if found != nil {
			return
		}
		if b, ok := o.(*widget.Button); ok && b.Text == text {
			found = b
		}
	})
	if found == nil {
		t.Fatalf("could not find a button labelled %q", text)
	}
	return found
}

// findComboByPlaceholder returns the searchComboBox whose entry carries the
// given placeholder. The editor has three of them — customers, saved items,
// and place of supply — so tests have to say which one they mean.
func findComboByPlaceholder(root any, placeholder string) *searchComboBox {
	var found *searchComboBox
	walk(root, func(o any) {
		if found != nil {
			return
		}
		if c, ok := o.(*searchComboBox); ok && c.entry.PlaceHolder == placeholder {
			found = c
		}
	})
	return found
}

// TestEditorWidget_CustomerPicker_SearchAndPickFillsForm drives the customer
// picker as a user does — type, then choose a row — rather than calling
// applyCustomerToForm directly. That wiring is what changed when the picker
// stopped being a widget.Select, and it is what a broken onPick would break
// while every state-level test kept passing.
func TestEditorWidget_CustomerPicker_SearchAndPickFillsForm(t *testing.T) {
	custs := []model.Customer{
		{Name: "Acme Corp", GSTIN: "33AAAAA0000A1Z5", BillingAddress: []string{"1 Main St"}},
		{Name: "Beta Traders", GSTIN: "27BBBBB1111B2Z6", BillingAddress: []string{"9 Side Rd"}},
	}
	ed := newTestEditorWithMasters(t, goldenState(), EditorMasters{Customers: custs})

	box := findComboByPlaceholder(ed.Container(), customerPickerPlaceholder)
	if box == nil {
		t.Fatal("no customer search box in the editor")
	}

	// Searching by GSTIN must narrow to the second customer.
	box.onQueryChanged("27BBBBB")
	if len(box.filtered) != 1 || box.filtered[0] != 1 {
		t.Fatalf("filtered = %v after searching a GSTIN, want just index 1 (Beta Traders)", box.filtered)
	}

	box.pick(box.filtered[0])

	if ed.custNameEntry.Text != "Beta Traders" {
		t.Errorf("name entry = %q, want Beta Traders", ed.custNameEntry.Text)
	}
	if ed.custGSTINEntry.Text != "27BBBBB1111B2Z6" {
		t.Errorf("GSTIN entry = %q, want the picked customer's", ed.custGSTINEntry.Text)
	}
	if ed.state.Customer.Name != "Beta Traders" {
		t.Errorf("state.Customer = %+v, want Beta Traders", ed.state.Customer)
	}
	// The box clears itself, so the same customer can be re-picked after a
	// manual tweak — the behaviour the old Select needed a reset hack for.
	if box.entry.Text != "" {
		t.Errorf("search box still reads %q after picking, want it cleared", box.entry.Text)
	}
}

func TestEditorWidget_PlaceOfSupply_SearchAndPick(t *testing.T) {
	ed, _ := newTestEditor(t, goldenState())

	box := findComboByPlaceholder(ed.Container(), placeOfSupplyPlaceholder)
	if box == nil {
		t.Fatal("no place-of-supply search box in the editor")
	}
	if box.clearOnPick {
		t.Error("place-of-supply combo should keep the picked value in the entry")
	}

	box.onQueryChanged("tamil")
	if len(box.filtered) != 1 {
		t.Fatalf("filtered = %v after searching tamil, want one Tamil Nadu row", box.filtered)
	}
	box.pick(box.filtered[0])

	if ed.state.PlaceOfSupply != "33-Tamil Nadu" {
		t.Errorf("PlaceOfSupply = %q, want 33-Tamil Nadu", ed.state.PlaceOfSupply)
	}
	if box.entry.Text != "33-Tamil Nadu" {
		t.Errorf("combo text = %q, want 33-Tamil Nadu", box.entry.Text)
	}
}

func TestEditorWidget_PlaceOfSupply_BareCodeExpanded(t *testing.T) {
	state := goldenState()
	state.PlaceOfSupply = "33"
	ed, _ := newTestEditor(t, state)
	if ed.posCombo.entry.Text != "33-Tamil Nadu" {
		t.Errorf("combo text = %q, want 33-Tamil Nadu expanded from a bare code", ed.posCombo.entry.Text)
	}
}

// TestEditorWidget_CustomerPicker_DuplicateNamesPickIndependently is the
// combobox restatement of what label disambiguation used to guarantee: with
// two customers named the same, choosing the second must apply the SECOND
// one's details, not the first match by name.
func TestEditorWidget_CustomerPicker_DuplicateNamesPickIndependently(t *testing.T) {
	custs := []model.Customer{
		{Name: "Acme", GSTIN: "33AAAAA0000A1Z5"},
		{Name: "Acme", GSTIN: "27BBBBB1111B2Z6"},
	}
	ed := newTestEditorWithMasters(t, goldenState(), EditorMasters{Customers: custs})

	box := findComboByPlaceholder(ed.Container(), customerPickerPlaceholder)
	if box == nil {
		t.Fatal("no customer search box in the editor")
	}

	box.onQueryChanged("Acme")
	if len(box.filtered) != 2 {
		t.Fatalf("filtered = %v, want both Acme rows", box.filtered)
	}
	box.pick(box.filtered[1])

	if got := ed.state.Customer.GSTIN; got != "27BBBBB1111B2Z6" {
		t.Errorf("picked the second Acme but got GSTIN %q, want the second one's", got)
	}
}
