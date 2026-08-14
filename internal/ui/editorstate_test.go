package ui

import (
	"testing"
	"time"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/testfixture"
)

func goldenState() *EditorState {
	return &EditorState{
		Type:          model.InvoiceExportLUT,
		Number:        "AEX2425-1",
		Date:          time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC),
		DueDate:       time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
		Customer:      testfixture.ExportCustomer(),
		PlaceOfSupply: "97-Other Territory",
		Currency:      "USD",
		FXFactor:      "83.2",
		Seller:        model.Company{StateCode: "33"},
		Items: []*LineItemState{
			{
				Description: "Software Development Services",
				HSNSAC:      "998314",
				Quantity:    "1.00",
				Unit:        "UNT",
				RateUSD:     "4000.00",
				DiscountUSD: "0.00",
				TaxRatePct:  "18",
				CessRatePct: "0",
			},
		},
	}
}

// TestNewEditorState_LayoutStyleFollowsSettingsUntilOverridden ensures new
// invoices do not stamp a layout choice up front — they follow Settings until
// the user changes Layout in the editor.
func TestNewEditorState_LayoutStyleFollowsSettingsUntilOverridden(t *testing.T) {
	classic := NewEditorState(model.Settings{LayoutStyle: model.LayoutClassic})
	if classic.LayoutStyle != "" {
		t.Errorf("LayoutStyle = %q, want empty so Settings classic applies until overridden", classic.LayoutStyle)
	}
	if got := model.EffectiveLayoutStyle(classic.LayoutStyle, model.LayoutClassic); got != model.LayoutClassic {
		t.Errorf("EffectiveLayoutStyle = %q, want classic from Settings", got)
	}

	blank := NewEditorState(model.Settings{})
	if blank.LayoutStyle != "" {
		t.Errorf("LayoutStyle = %q, want empty", blank.LayoutStyle)
	}
	if got := model.EffectiveLayoutStyle(blank.LayoutStyle, ""); got != model.LayoutModern {
		t.Errorf("EffectiveLayoutStyle = %q, want modern fallback", got)
	}
}

// TestEditorState_Build_CarriesLayoutStyle ensures the field the editor's
// layout select writes actually reaches the saved model.Invoice.
func TestEditorState_Build_CarriesLayoutStyle(t *testing.T) {
	s := goldenState()
	s.LayoutStyle = model.LayoutClassic
	inv, err := s.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if inv.LayoutStyle != model.LayoutClassic {
		t.Errorf("inv.LayoutStyle = %q, want classic", inv.LayoutStyle)
	}
}

func TestEditorState_Recalc_Golden(t *testing.T) {
	s := goldenState()
	result, err := s.Recalc()
	if err != nil {
		t.Fatalf("Recalc: %v", err)
	}
	if got, want := result.GrandTotal, money.Amount(33280000); got != want {
		t.Errorf("GrandTotal = %d, want %d", got, want)
	}
	if result.TotalTax != 0 {
		t.Errorf("TotalTax = %d, want 0 under LUT", result.TotalTax)
	}
	if got, want := result.TotalUSD, money.Amount(400000); got != want {
		t.Errorf("TotalUSD = %d, want %d", got, want)
	}
}

func TestEditorState_Recalc_ReflectsKeystrokeEdits(t *testing.T) {
	s := goldenState()
	before, err := s.Recalc()
	if err != nil {
		t.Fatalf("Recalc: %v", err)
	}

	// Simulate the user editing the rate field, as OnChanged would on every
	// keystroke.
	s.Items[0].RateUSD = "8000.00"
	after, err := s.Recalc()
	if err != nil {
		t.Fatalf("Recalc after edit: %v", err)
	}
	if after.GrandTotal == before.GrandTotal {
		t.Fatal("expected GrandTotal to change after editing the rate")
	}
	wantAfter := money.Amount(66560000) // 8000 * 83.2
	if after.GrandTotal != wantAfter {
		t.Errorf("GrandTotal after edit = %d, want %d", after.GrandTotal, wantAfter)
	}
}

func TestEditorState_Recalc_InvalidFieldReturnsError(t *testing.T) {
	s := goldenState()
	s.Items[0].RateUSD = "not-a-number"
	if _, err := s.Recalc(); err == nil {
		t.Fatal("expected an error for an unparseable rate")
	}
}

func TestEditorState_SetType_DomesticLocksCurrencyAndFactor(t *testing.T) {
	s := goldenState()
	if s.CurrencyLocked() {
		t.Fatal("export_lut must not start currency-locked")
	}

	s.SetType(model.InvoiceDomestic)
	if !s.CurrencyLocked() {
		t.Error("domestic must lock currency")
	}
	if s.Currency != "INR" {
		t.Errorf("Currency = %q, want INR after switching to domestic", s.Currency)
	}
	if s.FXFactor != "1" {
		t.Errorf("FXFactor = %q, want 1 after switching to domestic", s.FXFactor)
	}

	s.SetType(model.InvoiceExportIGST)
	if s.CurrencyLocked() {
		t.Error("export_igst must not be currency-locked")
	}
	if s.Currency != "USD" {
		t.Errorf("Currency = %q, want USD after switching back from domestic", s.Currency)
	}
}

func TestEditorState_SetType_ShippingFieldVisibility(t *testing.T) {
	s := goldenState()
	if !s.ShowShippingFields() {
		t.Error("export_lut should show shipping fields")
	}
	if s.ShippingRequired() {
		t.Error("export_lut should not require shipping fields (only export_igst does)")
	}

	s.SetType(model.InvoiceExportIGST)
	if !s.ShowShippingFields() || !s.ShippingRequired() {
		t.Error("export_igst should show and require shipping fields")
	}

	s.ShippingBillNo = "SB1"
	s.ShippingPortCode = "INMAA1"
	s.ShippingBillDate = "2024-05-01"
	s.SetType(model.InvoiceDomestic)
	if s.ShowShippingFields() {
		t.Error("domestic should not show shipping fields")
	}
	if s.ShippingBillNo != "" || s.ShippingPortCode != "" || s.ShippingBillDate != "" {
		t.Error("switching to domestic should clear stale shipping bill fields")
	}
}

func TestEditorState_Validate_RequiresCustomerNumberAndItems(t *testing.T) {
	s := &EditorState{Type: model.InvoiceExportLUT, FXFactor: "1"}
	errs := s.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation errors for an empty state")
	}
	joined := stringsJoin(errs)
	for _, want := range []string{"customer name", "invoice number", "line item"} {
		if !containsSubstring(joined, want) {
			t.Errorf("expected validation errors to mention %q, got: %v", want, errs)
		}
	}
}

func TestEditorState_Validate_RequiresShippingForIGSTOnly(t *testing.T) {
	s := goldenState()
	s.SetType(model.InvoiceExportIGST)
	errs := s.Validate()
	joined := stringsJoin(errs)
	if !containsSubstring(joined, "shipping bill no") {
		t.Errorf("expected a shipping bill no. error for export_igst, got: %v", errs)
	}

	s.ShippingBillNo = "SB1"
	s.ShippingPortCode = "INMAA1"
	s.ShippingBillDate = "2024-05-01"
	errs = s.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors once shipping fields are filled, got: %v", errs)
	}

	// LUT must not require them even when empty.
	s.SetType(model.InvoiceExportLUT)
	errs = s.Validate()
	if len(errs) != 0 {
		t.Errorf("LUT should not require shipping fields, got: %v", errs)
	}
}

func TestEditorState_AddAndRemoveLine(t *testing.T) {
	s := goldenState()
	if len(s.Items) != 1 {
		t.Fatalf("expected 1 seed item, got %d", len(s.Items))
	}
	s.AddLine("998314", "UNT", 1800)
	if len(s.Items) != 2 {
		t.Fatalf("expected 2 items after AddLine, got %d", len(s.Items))
	}

	target := s.Items[0]
	s.RemoveLineState(target)
	if len(s.Items) != 1 {
		t.Fatalf("expected 1 item after RemoveLineState, got %d", len(s.Items))
	}
	for _, it := range s.Items {
		if it == target {
			t.Fatal("RemoveLineState did not remove the targeted item")
		}
	}
}

func TestEditorState_RemoveLine_ByIndex(t *testing.T) {
	s := goldenState()
	s.AddLine("", "", 0)
	s.AddLine("", "", 0)
	if len(s.Items) != 3 {
		t.Fatalf("setup: expected 3 items, got %d", len(s.Items))
	}
	s.RemoveLine(1)
	if len(s.Items) != 2 {
		t.Errorf("expected 2 items after RemoveLine(1), got %d", len(s.Items))
	}
	// Out-of-range indices must be a no-op, not a panic.
	s.RemoveLine(-1)
	s.RemoveLine(99)
	if len(s.Items) != 2 {
		t.Errorf("out-of-range RemoveLine should be a no-op, got %d items", len(s.Items))
	}
}

// ---- CloneInvoice ---------------------------------------------------------

type fakeNumberer struct {
	calls []string
	next  string
	err   error
}

func (f *fakeNumberer) NextInvoiceNumber(pattern string, date time.Time) (string, error) {
	f.calls = append(f.calls, pattern)
	if f.err != nil {
		return "", f.err
	}
	return f.next, nil
}

func TestCloneInvoice_AllocatesNumberAndTodaysDate(t *testing.T) {
	orig := model.Invoice{
		ID:     42,
		Type:   model.InvoiceExportLUT,
		Number: "AEX2425-1",
		Date:   time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC),
		Customer: model.Customer{
			Name:           testfixture.ExportCustomerName,
			BillingAddress: []string{"1209 Orange Street"},
		},
		Items:       []model.LineItem{{Description: "Line 1"}},
		LayoutStyle: model.LayoutClassic,
	}
	fn := &fakeNumberer{next: "AEX2425-2"}
	today := time.Date(2024, 5, 30, 0, 0, 0, 0, time.UTC)

	clone, err := CloneInvoice(fn, "AEX{FY}-{SEQ}", orig, today)
	if err != nil {
		t.Fatalf("CloneInvoice: %v", err)
	}
	if clone.ID != 0 {
		t.Errorf("clone.ID = %d, want 0 (unsaved)", clone.ID)
	}
	if clone.Number != "AEX2425-2" {
		t.Errorf("clone.Number = %q, want AEX2425-2", clone.Number)
	}
	if !clone.Date.Equal(today) {
		t.Errorf("clone.Date = %v, want %v", clone.Date, today)
	}
	if clone.Customer.Name != orig.Customer.Name {
		t.Errorf("clone lost customer data: %+v", clone.Customer)
	}
	if len(fn.calls) != 1 || fn.calls[0] != "AEX{FY}-{SEQ}" {
		t.Errorf("expected exactly one NextInvoiceNumber call with the pattern, got %v", fn.calls)
	}
	if clone.LayoutStyle != model.LayoutClassic {
		t.Errorf("clone.LayoutStyle = %q, want classic preserved from original", clone.LayoutStyle)
	}

	// Deep-copy check: mutating the clone's items must not affect the
	// original.
	clone.Items[0].Description = "mutated"
	if orig.Items[0].Description == "mutated" {
		t.Error("CloneInvoice aliased the original Items slice")
	}
}

func TestCloneInvoice_NoPatternConfigured(t *testing.T) {
	fn := &fakeNumberer{next: "X"}
	_, err := CloneInvoice(fn, "", model.Invoice{Type: model.InvoiceDomestic}, time.Now())
	if err == nil {
		t.Fatal("expected an error when no numbering pattern is configured")
	}
	if len(fn.calls) != 0 {
		t.Error("should not call NextInvoiceNumber when pattern is empty")
	}
}

func TestEditorStateFromInvoice_RoundTrips(t *testing.T) {
	shipDate := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	inv := model.Invoice{
		Type:             model.InvoiceExportIGST,
		Number:           "AEXI2425-1",
		Date:             time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC),
		DueDate:          time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
		Customer:         model.Customer{Name: "Acme"},
		Currency:         "USD",
		FXFactor:         money.Rate(83_200_000),
		ShippingBillDate: &shipDate,
		Items: []model.LineItem{
			{Description: "X", Quantity: money.Qty(250), RateUSD: money.Amount(123456), DiscountUSD: money.Amount(500), TaxRatePct: 1850, CessRatePct: 100},
		},
		LayoutStyle: model.LayoutClassic,
	}

	s := EditorStateFromInvoice(inv)
	if s.FXFactor != "83.2" {
		t.Errorf("FXFactor = %q, want 83.2", s.FXFactor)
	}
	// Editing (or cloning, which also goes through EditorStateFromInvoice)
	// must preserve the layout the invoice was saved with, not silently
	// reset it to the Settings default.
	if s.LayoutStyle != model.LayoutClassic {
		t.Errorf("LayoutStyle = %q, want classic preserved from the saved invoice", s.LayoutStyle)
	}
	if s.ShippingBillDate != "2024-05-01" {
		t.Errorf("ShippingBillDate = %q, want 2024-05-01", s.ShippingBillDate)
	}
	if len(s.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(s.Items))
	}
	li := s.Items[0]
	if li.Quantity != "2.50" {
		t.Errorf("Quantity = %q, want 2.50", li.Quantity)
	}
	if li.RateUSD != "1234.56" {
		t.Errorf("RateUSD = %q, want 1234.56", li.RateUSD)
	}
	if li.TaxRatePct != "18.50" {
		t.Errorf("TaxRatePct = %q, want 18.50", li.TaxRatePct)
	}

	// Build() must reproduce the same computed figures as the original.
	rebuilt, err := s.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if rebuilt.Items[0].RateUSD != inv.Items[0].RateUSD {
		t.Errorf("round-tripped RateUSD = %d, want %d", rebuilt.Items[0].RateUSD, inv.Items[0].RateUSD)
	}
	if rebuilt.Items[0].TaxRatePct != inv.Items[0].TaxRatePct {
		t.Errorf("round-tripped TaxRatePct = %d, want %d", rebuilt.Items[0].TaxRatePct, inv.Items[0].TaxRatePct)
	}
}

// TestEditorStateFromInvoice_PreservesEmptyLayoutStyle covers a legacy
// invoice saved before LayoutStyle existed: its zero value ("") must stay
// empty so re-export keeps following Settings instead of locking in Modern
// the first time the invoice is opened for edit.
func TestEditorStateFromInvoice_PreservesEmptyLayoutStyle(t *testing.T) {
	inv := model.Invoice{Type: model.InvoiceDomestic, Items: []model.LineItem{{Description: "X"}}}
	s := EditorStateFromInvoice(inv)
	if s.LayoutStyle != "" {
		t.Errorf("LayoutStyle = %q, want empty preserved for a legacy invoice with no stored layout", s.LayoutStyle)
	}
}

// ---- small local string helpers (kept test-local to avoid pulling in a
// dependency just for two substring checks) --------------------------------

func stringsJoin(errs []string) string {
	out := ""
	for i, e := range errs {
		if i > 0 {
			out += " | "
		}
		out += e
	}
	return out
}

func containsSubstring(haystack, needle string) bool {
	hl, nl := len(haystack), len(needle)
	if nl == 0 {
		return true
	}
	toLower := func(b byte) byte {
		if b >= 'A' && b <= 'Z' {
			return b + 32
		}
		return b
	}
	for i := 0; i+nl <= hl; i++ {
		match := true
		for j := 0; j < nl; j++ {
			if toLower(haystack[i+j]) != toLower(needle[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
