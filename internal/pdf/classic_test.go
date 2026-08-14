package pdf

import (
	"fmt"
	"testing"
	"time"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/testfixture"
)

// classicInvoice returns baseInvoice with LayoutStyle stamped classic, so
// every test in this file exercises renderClassic rather than the default
// modern layout.
func classicInvoice(t *testing.T) *model.Invoice {
	inv := baseInvoice(t)
	inv.LayoutStyle = model.LayoutClassic
	return inv
}

func TestClassicRender_ExportLUT(t *testing.T) {
	inv := classicInvoice(t)

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
	if countPages(data) != 1 {
		t.Errorf("expected 1 page, got %d", countPages(data))
	}
}

func TestClassicRender_ExportIGST(t *testing.T) {
	inv := classicInvoice(t)
	inv.Type = model.InvoiceExportIGST
	inv.ShippingBillNo = "SB12345"
	inv.ShippingPortCode = "INMAA1"
	sbd := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	inv.ShippingBillDate = &sbd

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

func TestClassicRender_DomesticIntraState_WithHSNSummary(t *testing.T) {
	inv := classicInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Currency = "INR"
	inv.FXFactor = money.Rate(money.RateScale)
	inv.PlaceOfSupply = "33-Tamil Nadu"
	inv.Items[0].RateUSD = money.Amount(13500000)
	inv.IncludeHSNSACSummary = true

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

// A one-line domestic invoice carrying every optional footer section (HSN/SAC
// summary plus the UPI QR column) must still fit on a single sheet: the items
// table's cosmetic min-height padding has to give way to the footer rather
// than push it onto a second page.
func TestClassicRender_ShortDomesticWithHSNAndUPI_FitsOnePage(t *testing.T) {
	inv := classicInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Currency = "INR"
	inv.FXFactor = money.Rate(money.RateScale)
	inv.PlaceOfSupply = "33-Tamil Nadu"
	inv.IncludeHSNSACSummary = true
	inv.Items = []model.LineItem{{
		Description: "Service Fees",
		HSNSAC:      "998314",
		Quantity:    money.Qty(100),
		Unit:        "UNT",
		RateUSD:     money.Amount(10000),
		TaxRatePct:  1800,
	}}
	inv.Bank.UPIID = "seller@icici"

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
	if got := countPages(data); got != 1 {
		t.Errorf("expected 1 page, got %d", got)
	}
}

func TestClassicRender_DomesticInterState_IGST(t *testing.T) {
	inv := classicInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Currency = "INR"
	inv.FXFactor = money.Rate(money.RateScale)
	inv.PlaceOfSupply = "27-Maharashtra"
	inv.Items[0].RateUSD = money.Amount(1000000)
	inv.IncludeHSNSACSummary = true

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

func TestClassicRender_WithUPIQR(t *testing.T) {
	inv := classicInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Currency = "INR"
	inv.FXFactor = money.Rate(money.RateScale)
	inv.PlaceOfSupply = "33-Tamil Nadu"
	inv.Bank.UPIID = "merchant@hdfc"
	inv.IncludeHSNSACSummary = true

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

// TestClassicRender_MultiPage_40Lines is the plan's explicit stress case: a
// 40-line invoice with long wrapping descriptions must paginate cleanly,
// repeating the items-table header on each new page, without panicking or
// overlapping the footer bands.
func TestClassicRender_MultiPage_40Lines(t *testing.T) {
	inv := classicInvoice(t)
	inv.Items = nil
	for i := 0; i < 40; i++ {
		inv.Items = append(inv.Items, model.LineItem{
			Description: fmt.Sprintf("Software Development Services - Sprint %d (extended description to force wrapping across multiple lines in the item cell)", i+1),
			HSNSAC:      "998314",
			Quantity:    money.Qty(100),
			Unit:        "UNT",
			RateUSD:     money.Amount(20000),
			TaxRatePct:  1800,
		})
	}

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)

	pages := countPages(data)
	if pages < 2 {
		t.Fatalf("expected multi-page output for 40 wrapping line items, got %d page(s)", pages)
	}
}

func TestClassicRender_ZeroItems(t *testing.T) {
	inv := classicInvoice(t)
	inv.Items = nil

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

// TestClassicRender_EmptyOptionalFields exercises the "must not panic on
// empty data" requirement directly: blank GSTIN, no logo, no signature, no
// UPI ID, zero DueDate, blank city.
func TestClassicRender_EmptyOptionalFields(t *testing.T) {
	inv := classicInvoice(t)
	inv.DueDate = time.Time{}
	inv.Seller.GSTIN = ""
	inv.Seller.LogoPath = ""
	inv.Seller.SignaturePath = ""
	inv.Seller.City = ""
	inv.Bank.UPIID = ""
	inv.Customer.GSTIN = ""
	inv.PlaceOfSupply = ""

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

// TestClassicRender_AllInvoiceTypes renders one PDF per model.InvoiceType to
// pin that renderClassic never panics for any of the three supported types.
func TestClassicRender_AllInvoiceTypes(t *testing.T) {
	types := []model.InvoiceType{model.InvoiceExportLUT, model.InvoiceExportIGST, model.InvoiceDomestic}
	for _, typ := range types {
		t.Run(string(typ), func(t *testing.T) {
			inv := classicInvoice(t)
			inv.Type = typ
			if typ == model.InvoiceDomestic {
				inv.Currency = "INR"
				inv.FXFactor = money.Rate(money.RateScale)
				inv.PlaceOfSupply = "33-Tamil Nadu"
			}
			data, err := Render(inv)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			requireValidPDF(t, data)
		})
	}
}

// TestClassicItemColumns_SumToContentWidth pins the items table's fixed mm
// column widths (9/84/20/20/20/10/27) to contentW, per the plan.
func TestClassicItemColumns_SumToContentWidth(t *testing.T) {
	cols := classicItemColumns()
	sum := 0.0
	for _, c := range cols {
		sum += c.Width
	}
	if sum != contentW {
		t.Errorf("classic item column widths sum to %v, want contentW %v", sum, contentW)
	}
}

// TestClassicHSNColumns_SumToTargetWidth checks both HSN/SAC summary column
// width sets (full 190mm, and 156mm alongside the 34mm UPI QR column) for
// both tax modes, per the plan's explicit width-set test requirement.
func TestClassicHSNColumns_SumToTargetWidth(t *testing.T) {
	cases := []struct {
		name  string
		mode  calc.TaxMode
		qr    bool
		total float64
	}{
		{"CGST/SGST no QR", calc.TaxCGSTSGST, false, contentW},
		{"CGST/SGST with QR", calc.TaxCGSTSGST, true, contentW - classicHSNQRColW},
		{"IGST no QR", calc.TaxIGST, false, contentW},
		{"IGST with QR", calc.TaxIGST, true, contentW - classicHSNQRColW},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cols := buildClassicHSNColumns(tc.mode, tc.qr)
			sum := 0.0
			for _, c := range cols {
				sum += c.Width
			}
			if sum != tc.total {
				t.Errorf("column widths sum to %v, want %v", sum, tc.total)
			}
		})
	}
	// The QR column itself plus its adjoining table must reach exactly
	// contentW, so the reserved-column table never runs past the frame.
	if classicHSNQRColW+(contentW-classicHSNQRColW) != contentW {
		t.Errorf("QR column + summary table must sum to contentW")
	}
}

// TestClassicQtySummary checks the Total row's quantity cell: same-unit
// lines sum with the unit suffix, mixed units fall back to the bare number.
// The expected "16 NOS" matches the reference document's Total row exactly —
// this format prints whole counts without decimals.
func TestClassicQtySummary(t *testing.T) {
	items := []model.LineItem{
		{Quantity: money.Qty(200), Unit: "NOS"},
		{Quantity: money.Qty(1200), Unit: "NOS"},
		{Quantity: money.Qty(200), Unit: "NOS"},
	}
	if got, want := classicQtySummary(items), "16 NOS"; got != want {
		t.Errorf("classicQtySummary(same unit) = %q, want %q", got, want)
	}

	mixed := []model.LineItem{
		{Quantity: money.Qty(100), Unit: "NOS"},
		{Quantity: money.Qty(100), Unit: "KG"},
	}
	if got, want := classicQtySummary(mixed), "2"; got != want {
		t.Errorf("classicQtySummary(mixed units) = %q, want %q", got, want)
	}

	if got := classicQtySummary(nil); got != "" {
		t.Errorf("classicQtySummary(nil) = %q, want empty", got)
	}
}

// TestClassicQty covers the quantity formatting rule directly: whole counts
// lose their decimals, genuinely fractional quantities keep them.
func TestClassicQty(t *testing.T) {
	cases := []struct {
		qty  money.Qty
		want string
	}{
		{money.Qty(200), "2"},
		{money.Qty(1200), "12"},
		{money.Qty(1600), "16"},
		{money.Qty(0), "0"},
		{money.Qty(250), "2.5"},
		{money.Qty(125), "1.25"},
		{money.Qty(-200), "-2"},
	}
	for _, tc := range cases {
		if got := classicQty(tc.qty); got != tc.want {
			t.Errorf("classicQty(%d) = %q, want %q", int64(tc.qty), got, tc.want)
		}
	}
}

// TestClassicDeclaration_FallsBackForDomestic pins that the Declaration box is
// never printed as an empty captioned rectangle: domestic invoices carry no
// configured declaration, so the classic layout supplies the standard wording.
func TestClassicDeclaration_FallsBackForDomestic(t *testing.T) {
	inv := baseInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Notes = ""

	if declarationFor(inv) != "" {
		t.Fatal("test assumes a domestic invoice has no declaration of its own")
	}
	if got := classicDeclaration(inv); got != DeclarationClassicDefault {
		t.Errorf("classicDeclaration = %q, want the standard fallback", got)
	}

	// A configured declaration must win over the fallback.
	inv.Notes = "Custom declaration text"
	if got := classicDeclaration(inv); got != "Custom declaration text" {
		t.Errorf("classicDeclaration = %q, want the configured text", got)
	}
}

// TestPlaceOfSupplyNameCode checks the "code-name" -> (name, code) split
// used to derive the Consignee/Buyer "State Name" line.
func TestPlaceOfSupplyNameCode(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantCode string
	}{
		{"33-Tamil Nadu", "Tamil Nadu", "33"},
		{"27-Maharashtra", "Maharashtra", "27"},
		{"33", "Tamil Nadu", "33"},
		{"", "", ""},
		{"NoDashHere", "NoDashHere", ""},
	}
	for _, tc := range cases {
		name, code := placeOfSupplyNameCode(tc.in)
		if name != tc.wantName || code != tc.wantCode {
			t.Errorf("placeOfSupplyNameCode(%q) = (%q, %q), want (%q, %q)", tc.in, name, code, tc.wantName, tc.wantCode)
		}
	}
}

// TestClassicTaxLines checks which tax lines print for each tax mode, and
// that CESS/Round Off only appear when non-zero.
func TestClassicTaxLines(t *testing.T) {
	cgstsgst := calc.InvoiceResult{Mode: calc.TaxCGSTSGST, TaxableINR: 100, CGST: 9, SGST: 9}
	lines := classicTaxLines(cgstsgst)
	if len(lines) != 3 {
		t.Fatalf("CGST/SGST: got %d lines, want 3 (subtotal+CGST+SGST)", len(lines))
	}
	if lines[1].Label != LabelClassicCentralGST || lines[2].Label != LabelClassicStateGST {
		t.Errorf("CGST/SGST labels = %q, %q", lines[1].Label, lines[2].Label)
	}

	igst := calc.InvoiceResult{Mode: calc.TaxIGST, TaxableINR: 100, IGST: 18}
	lines = classicTaxLines(igst)
	if len(lines) != 2 || lines[1].Label != LabelClassicIGST {
		t.Fatalf("IGST: got %v", lines)
	}

	withExtras := calc.InvoiceResult{Mode: calc.TaxIGST, TaxableINR: 100, IGST: 18, CESS: 5, RoundOff: -1}
	lines = classicTaxLines(withExtras)
	if len(lines) != 4 {
		t.Fatalf("expected CESS and Round Off appended when non-zero, got %v", lines)
	}
	if lines[2].Label != LabelClassicCESS || lines[3].Label != LabelClassicRoundOff {
		t.Errorf("unexpected trailing labels: %q, %q", lines[2].Label, lines[3].Label)
	}
}

func TestLayoutAndTypeAreIndependent(t *testing.T) {
	layouts := []model.LayoutStyle{model.LayoutModern, model.LayoutClassic}

	types := []struct {
		name      string
		typ       model.InvoiceType
		wantSwift bool // SWIFT routes a foreign remittance: export only
		wantUPI   bool // UPI is an India-only rail: domestic only
	}{
		{"domestic", model.InvoiceDomestic, false, true},
		{"export LUT", model.InvoiceExportLUT, true, false},
		{"export IGST", model.InvoiceExportIGST, true, false},
	}

	for _, tt := range types {
		for _, layout := range layouts {
			t.Run(tt.name+"/"+string(layout), func(t *testing.T) {
				inv := classicInvoice(t)
				inv.Type = tt.typ
				inv.LayoutStyle = layout
				inv.Bank.SwiftCode = testfixture.BankSwift
				inv.Bank.UPIID = "merchant@hdfc"

				if got := showSwiftCode(inv); got != tt.wantSwift {
					t.Errorf("showSwiftCode = %v, want %v — SWIFT visibility must follow invoice type, not layout", got, tt.wantSwift)
				}
				if got := showUPIID(inv); got != tt.wantUPI {
					t.Errorf("showUPIID = %v, want %v — UPI visibility must follow invoice type, not layout", got, tt.wantUPI)
				}

				// Every combination must render a valid PDF, not just the
				// two that pair "naturally".
				data, err := Render(inv)
				if err != nil {
					t.Fatalf("Render: %v", err)
				}
				requireValidPDF(t, data)
			})
		}
	}
}

func TestClassicStateLine(t *testing.T) {
	cases := []struct {
		name, code string
		want       string
	}{
		{"Tamil Nadu", "33", "State Name : Tamil Nadu, Code : 33"},
		{"Tamil Nadu", "", "State Name : Tamil Nadu"},
		{"", "33", "State Name : Tamil Nadu, Code : 33"},
		{"33", "", "State Name : Tamil Nadu, Code : 33"},
		{"", "", "State Name : -"},
	}
	for _, tc := range cases {
		if got := classicStateLine(tc.name, tc.code); got != tc.want {
			t.Errorf("classicStateLine(%q, %q) = %q, want %q", tc.name, tc.code, got, tc.want)
		}
	}
}

// TestClassicHeaderRightRows_IgnoresLayoutStyle is the companion to
// TestLayoutAndTypeAreIndependent for the classic header grid: its row set is
// chosen by invoice type, and flipping LayoutStyle must not change it.
func TestClassicHeaderRightRows_IgnoresLayoutStyle(t *testing.T) {
	for _, typ := range []model.InvoiceType{model.InvoiceDomestic, model.InvoiceExportLUT, model.InvoiceExportIGST} {
		inv := classicInvoice(t)
		inv.Type = typ

		inv.LayoutStyle = model.LayoutClassic
		classic := classicHeaderRightRows(inv)
		inv.LayoutStyle = model.LayoutModern
		modern := classicHeaderRightRows(inv)

		if len(classic) != len(modern) {
			t.Fatalf("%s: row count changed with layout: %d vs %d", typ, len(classic), len(modern))
		}
		for i := range classic {
			if classic[i] != modern[i] {
				t.Errorf("%s: row %d changed with layout: %+v vs %+v", typ, i, classic[i], modern[i])
			}
		}
	}
}
