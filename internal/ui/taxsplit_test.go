package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
)

// TestEditorState_DomesticSplitFollowsGSTINStates exercises the intra-state
// rule through EditorState.Recalc — the same path the editor's summary rail
// updates from on every keystroke.
//
// internal/calc already covers the rule itself against a hand-built
// model.Invoice. What that cannot catch is the wiring: the seller's GSTIN
// reaches the tax decision only if ApplySettings stamps Settings.Company onto
// the editor state and Build carries it through to the invoice. Break either
// link and the split silently falls back to IGST for every domestic invoice,
// with nothing in the calc tests going red.
func TestEditorState_DomesticSplitFollowsGSTINStates(t *testing.T) {
	test.NewTempApp(t)

	settings := model.Settings{
		Company:  model.Company{GSTIN: "33AAECA1234A1Z9", StateCode: "33"},
		Defaults: model.Defaults{TaxRatePct: 1800, Unit: "UNT", HSNSAC: "998314"},
	}

	cases := []struct {
		name      string
		custGSTIN string
		wantMode  calc.TaxMode
	}{
		{"buyer in the seller's state", "33AAAAA0000A1Z5", calc.TaxCGSTSGST},
		{"buyer in another state", "27BBBBB1111B2Z6", calc.TaxIGST},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := NewEditorState(settings)
			st.ApplySettings(settings)
			st.SetType(model.InvoiceDomestic)
			st.Customer.Name = "Buyer"
			st.Customer.GSTIN = c.custGSTIN
			st.Items[0].Description = "Consulting"
			st.Items[0].RateUSD = "10000.00"

			res, err := st.Recalc()
			if err != nil {
				t.Fatalf("Recalc: %v", err)
			}
			if res.Mode != c.wantMode {
				t.Fatalf("Mode = %v, want %v", res.Mode, c.wantMode)
			}

			if c.wantMode == calc.TaxCGSTSGST {
				if res.CGST == 0 || res.CGST != res.SGST {
					t.Errorf("want an equal, non-zero CGST/SGST split, got %d/%d", res.CGST, res.SGST)
				}
				if res.IGST != 0 {
					t.Errorf("IGST = %d, want 0 on an intra-state supply", res.IGST)
				}
			} else {
				if res.IGST == 0 {
					t.Error("IGST = 0, want the full rate charged as IGST")
				}
				if res.CGST != 0 || res.SGST != 0 {
					t.Errorf("CGST/SGST = %d/%d, want 0/0 on an inter-state supply", res.CGST, res.SGST)
				}
			}

			// The split must never change what the buyer actually owes.
			if got, want := res.TotalTax, res.TaxableINR*18/100; got != want {
				t.Errorf("TotalTax = %d, want %d (18%% however it is split)", got, want)
			}
		})
	}
}

// TestEditor_DomesticHidesCrossBorderFields covers the type-switch rule for
// the export-only supply fields. Country of supply and the currency/FX pair
// are meaningless on a domestic invoice — currency is locked to INR at factor
// 1 — so they are hidden rather than shown greyed out, and a country typed on
// an export invoice must not survive a switch to domestic.
func TestEditor_DomesticHidesCrossBorderFields(t *testing.T) {
	test.NewTempApp(t)
	settings := model.Settings{Defaults: model.Defaults{TaxRatePct: 1800, Currency: "USD"}}

	st := NewEditorState(settings)
	ed := NewEditor(st, settings, EditorMasters{}, func(model.Invoice) error { return nil }, func() {})

	// Starts on an export type: all three visible.
	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceExportLUT))
	for name, o := range map[string]interface{ Visible() bool }{
		"country": ed.countryField, "currency": ed.currencyField, "fx": ed.fxField,
	} {
		if !o.Visible() {
			t.Errorf("%s field should be visible on an export invoice", name)
		}
	}
	ed.countryEntry.SetText("UNITED STATES OF AMERICA")

	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceDomestic))
	for name, o := range map[string]interface{ Visible() bool }{
		"country": ed.countryField, "currency": ed.currencyField, "fx": ed.fxField,
	} {
		if o.Visible() {
			t.Errorf("%s field should be hidden on a domestic invoice", name)
		}
	}
	if !ed.supplySpacer.Visible() {
		t.Error("the spacer should hold the hidden fields' place in the row")
	}
	if st.CountryOfSupply != "" {
		t.Errorf("CountryOfSupply = %q, want it cleared on switching to domestic", st.CountryOfSupply)
	}
	if ed.countryEntry.Text != "" {
		t.Errorf("country widget = %q, want it resynced to the cleared state", ed.countryEntry.Text)
	}
	if st.Currency != "INR" || st.FXFactor != "1" {
		t.Errorf("domestic should be INR at factor 1, got %q / %q", st.Currency, st.FXFactor)
	}

	// Switching back restores them.
	ed.typeSelect.SetSelected(InvoiceTypeLabel(model.InvoiceExportIGST))
	if !ed.countryField.Visible() || !ed.currencyField.Visible() || !ed.fxField.Visible() {
		t.Error("cross-border fields should return on an export invoice")
	}
	if ed.supplySpacer.Visible() {
		t.Error("the spacer should stand down when the real fields return")
	}
}
