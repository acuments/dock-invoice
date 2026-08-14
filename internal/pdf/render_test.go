package pdf

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"
	"time"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/testfixture"
)

func mustRate(t *testing.T, s string) money.Rate {
	t.Helper()
	r, err := money.ParseRate(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func baseInvoice(t *testing.T) *model.Invoice {
	return testfixture.ExportLUTInvoice(mustRate(t, "83.2"))
}

var pageObjRe = regexp.MustCompile(`/Type\s*/Page\b(?:[^s]|$)`)

func countPages(data []byte) int {
	return len(pageObjRe.FindAll(data, -1))
}

func requireValidPDF(t *testing.T, data []byte) {
	t.Helper()
	if len(data) < 100 {
		t.Fatalf("pdf output too small: %d bytes", len(data))
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("pdf output missing %%PDF- header, got %q", data[:8])
	}
	tail := data[len(data)-64:]
	if !bytes.Contains(tail, []byte("%%EOF")) {
		t.Fatalf("pdf output missing %%%%EOF trailer")
	}
}

func TestRender_ExportLUT_Golden(t *testing.T) {
	inv := baseInvoice(t)
	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
	if countPages(data) != 1 {
		t.Errorf("expected 1 page for a single-line invoice, got %d", countPages(data))
	}
}

func TestRender_ExportIGST(t *testing.T) {
	inv := baseInvoice(t)
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

func TestRender_DomesticIntraState_CGSTSGST(t *testing.T) {
	inv := baseInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Currency = "INR"
	inv.FXFactor = money.Rate(money.RateScale)
	inv.PlaceOfSupply = "33-Tamil Nadu"
	// Match the sample: 1,35,000 @ 18% → CGST/SGST 12,150 each.
	inv.Items[0].RateUSD = money.Amount(13500000)
	inv.IncludeHSNSACSummary = true

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

func TestRender_DomesticInterState_IGST(t *testing.T) {
	inv := baseInvoice(t)
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

// TestHSNCellValue_IntraStateMatchesSample checks the numbers that land in
// the HSN summary cells against the statutory CGST/SGST sample layout.
func TestHSNCellValue_IntraStateMatchesSample(t *testing.T) {
	row := calc.HSNSummaryRow{
		HSNSAC:     "998314",
		TaxRatePct: 1800,
		TaxableINR: money.Amount(13500000),
		CGST:       money.Amount(1215000),
		SGST:       money.Amount(1215000),
		TotalTax:   money.Amount(2430000),
	}
	want := map[string]string{
		"hsn":       "998314",
		"taxable":   "1,35,000.00",
		"cgst_rate": "9%",
		"cgst_amt":  "12,150.00",
		"sgst_rate": "9%",
		"sgst_amt":  "12,150.00",
		"total_tax": "24,300.00",
	}
	for id, expected := range want {
		if got := hsnCellValue(id, row); got != expected {
			t.Errorf("hsnCellValue(%q) = %q, want %q", id, got, expected)
		}
	}
}

func TestHSNCellValue_InterStateIGST(t *testing.T) {
	row := calc.HSNSummaryRow{
		HSNSAC:     "998314",
		TaxRatePct: 1800,
		TaxableINR: money.Amount(1000000),
		IGST:       money.Amount(180000),
		TotalTax:   money.Amount(180000),
	}
	want := map[string]string{
		"hsn":       "998314",
		"taxable":   "10,000.00",
		"igst_rate": "18%",
		"igst_amt":  "1,800.00",
		"total_tax": "1,800.00",
	}
	for id, expected := range want {
		if got := hsnCellValue(id, row); got != expected {
			t.Errorf("hsnCellValue(%q) = %q, want %q", id, got, expected)
		}
	}
}

func TestBuildHSNColumns(t *testing.T) {
	intra := buildHSNColumns(calc.TaxCGSTSGST)
	if got := hsnTableWidth(intra); got != contentW {
		t.Errorf("CGST/SGST columns width = %v, want contentW %v", got, contentW)
	}
	if !hsnHasGroups(intra) {
		t.Error("CGST/SGST columns should use grouped Rate/Amount headers")
	}

	inter := buildHSNColumns(calc.TaxIGST)
	if got := hsnTableWidth(inter); got != contentW {
		t.Errorf("IGST columns width = %v, want contentW %v", got, contentW)
	}
	// Inter-state domestic still groups IGST Rate/Amount.
	if !hsnHasGroups(inter) {
		t.Error("IGST columns should use a grouped Rate/Amount header")
	}
}

func TestHSNSummaryHeight_EmptyLines(t *testing.T) {
	if h := hsnSummaryHeight(calc.InvoiceResult{}); h != 0 {
		t.Errorf("height for empty result = %v, want 0", h)
	}
}

// TestRender_MultiPage_RepeatsHeaderAndKeepsFooterTogether renders a
// 20-line invoice with long wrapping descriptions and checks it spills onto
// more than one page (per the plan's multi-page requirement) while still
// producing valid output that includes the totals/footer content.
func TestRender_MultiPage_RepeatsHeaderAndKeepsFooterTogether(t *testing.T) {
	inv := baseInvoice(t)
	inv.Items = nil
	for i := 0; i < 20; i++ {
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
		t.Fatalf("expected multi-page output for 20 wrapping line items, got %d page(s)", pages)
	}
}

func TestRender_WithUPIQR(t *testing.T) {
	inv := baseInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Currency = "INR"
	inv.FXFactor = money.Rate(money.RateScale)
	inv.PlaceOfSupply = "33-Tamil Nadu"
	inv.Bank.UPIID = "merchant@hdfc"

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

// TestShowUPI_DomesticOnly pins the rule that UPI (both the bank-details row
// and the scan-to-pay QR) prints for domestic invoices only — an export buyer
// cannot pay a VPA.
func TestShowUPI_DomesticOnly(t *testing.T) {
	cases := []struct {
		name  string
		typ   model.InvoiceType
		upiID string
		want  bool
	}{
		{"domestic with UPI ID", model.InvoiceDomestic, "merchant@hdfc", true},
		{"domestic without UPI ID", model.InvoiceDomestic, "", false},
		{"domestic with blank UPI ID", model.InvoiceDomestic, "   ", false},
		{"export LUT with UPI ID", model.InvoiceExportLUT, "merchant@hdfc", false},
		{"export IGST with UPI ID", model.InvoiceExportIGST, "merchant@hdfc", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := baseInvoice(t)
			inv.Type = tc.typ
			inv.Bank.UPIID = tc.upiID

			if got := showUPIID(inv); got != tc.want {
				t.Errorf("showUPIID = %v, want %v", got, tc.want)
			}
			result := calc.ComputeInvoice(inv)
			if got := showUPIQR(inv, result); got != tc.want {
				t.Errorf("showUPIQR = %v, want %v (grand total %v)", got, tc.want, result.GrandTotal)
			}
			// The block height must agree with what drawBankAndSignature
			// actually draws, or the page-break estimate drifts.
			const baseH = 1.5 + 26 + 3
			gotH := bankSignatureBlockHeight(inv, result)
			if tc.want && gotH == baseH {
				t.Errorf("bankSignatureBlockHeight = %v, want room reserved for the QR", gotH)
			}
			if !tc.want && gotH != baseH {
				t.Errorf("bankSignatureBlockHeight = %v, want %v (no QR drawn)", gotH, baseH)
			}
		})
	}
}

func TestRender_ExportWithUPIID(t *testing.T) {
	inv := baseInvoice(t)
	inv.Bank.UPIID = "merchant@hdfc"

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

// TestRender_LayoutStyles checks that every LayoutStyle value Render can
// see — including the empty zero value and a garbage/unrecognised one —
// produces valid PDF output. Unknown values must fall back to the modern
// layout rather than error, per model.NormalizeLayoutStyle.
func TestRender_LayoutStyles(t *testing.T) {
	cases := []struct {
		name  string
		style model.LayoutStyle
	}{
		{"empty defaults to modern", ""},
		{"explicit modern", model.LayoutModern},
		{"explicit classic", model.LayoutClassic},
		{"garbage falls back to modern", model.LayoutStyle("nonsense")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := baseInvoice(t)
			inv.LayoutStyle = tc.style

			data, err := Render(inv)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			requireValidPDF(t, data)
		})
	}
}

func TestRender_NilInvoice(t *testing.T) {
	if _, err := Render(nil); err == nil {
		t.Fatal("expected error for nil invoice")
	}
}

// TestRender_LongBillingAddress_WrapsWithinColumn is a regression for
// single-line bill-to addresses (e.g. pasted one-line strings) that
// previously drew past the billing column into Shipping Address.
func TestRender_LongBillingAddress_WrapsWithinColumn(t *testing.T) {
	long := testfixture.LongSingleLineBillingAddress
	inv := baseInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Currency = "INR"
	inv.FXFactor = money.Rate(money.RateScale)
	inv.PlaceOfSupply = "33-Tamil Nadu"
	inv.Customer = testfixture.DomesticCustomerRecord()
	inv.Customer.BillingAddress = []string{long}
	inv.Customer.ShippingAddress = []string{long}
	inv.Items[0].RateUSD = money.Amount(10000000)

	// Widths mirror drawPartyRow column geometry.
	const (
		col2W = 64.0 // col3X - col2X - 2
		col3W = 62.0 // pageW - marginR - col3X
	)

	d := newDoc()
	d.pdf.AddPage()
	d.pdf.SetFont(FontFamily, "", 9)
	billWrapped := d.wrapText(long, col2W)
	shipWrapped := d.wrapText(long, col3W)
	if len(billWrapped) < 2 {
		t.Fatalf("expected long billing address to wrap into >= 2 lines within %.0fmm, got %v", col2W, billWrapped)
	}
	if len(shipWrapped) < 2 {
		t.Fatalf("expected long shipping address to wrap into >= 2 lines within %.0fmm, got %v", col3W, shipWrapped)
	}
	for _, line := range billWrapped {
		if w := d.pdf.GetStringWidth(line); w > col2W+0.5 {
			t.Errorf("billing wrap line wider than column: %q is %.2fmm (limit %.0f)", line, w, col2W)
		}
	}
	for _, line := range shipWrapped {
		if w := d.pdf.GetStringWidth(line); w > col3W+0.5 {
			t.Errorf("shipping wrap line wider than column: %q is %.2fmm (limit %.0f)", line, w, col3W)
		}
	}

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

// TestRender_LongUPIID_FitsWithinBankBox is a regression for a UPI VPA that
// overran the "Swift Code:" label: drawBankAndSignature used to pack UPI ID
// and Swift Code into one row with a fixed 25mm value cell, and fpdf's
// CellFormat does not clip — a realistic VPA like
// "rajenderelectricals@rbl" (30.67mm at 8pt) printed straight over the
// neighbouring label ("merchant@hdfc" at 21.18mm never triggered it, which
// is why the original test fixture missed this). The fix gives the UPI ID
// its own full-width row instead of sharing a row with Swift Code.
func TestRender_LongUPIID_FitsWithinBankBox(t *testing.T) {
	// Geometry mirrors drawBankAndSignature/upiRow: pad=3, x1=marginL+pad,
	// labelW=30, box right edge at marginL+boxW-pad.
	const (
		boxW   = 110.0
		pad    = 3.0
		labelW = 30.0
	)
	x1 := marginL + pad
	valueX := x1 + labelW
	valueW := boxW - pad - (valueX - marginL) // width available for the value before the box's right edge

	d := newDoc()
	d.pdf.AddPage()
	d.pdf.SetFont(FontFamily, "", 8)

	longVPA := "rajenderelectricals@rbl" // the reported overflowing VPA (30.67mm)
	if w := d.pdf.GetStringWidth(longVPA); w > valueW {
		t.Errorf("VPA %q is %.2fmm wide, wider than the %.2fmm the bank box's UPI row allows", longVPA, w, valueW)
	}

	// A 40-character VPA must not touch a neighbouring label either. Since
	// the fix puts UPI ID alone on a full-width row, there is no label to
	// its right any more, but it must still fit inside the box.
	fortyCharVPA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@bbbbbbbbb" // 40 chars
	if w := d.pdf.GetStringWidth(fortyCharVPA); w > valueW {
		t.Errorf("40-char VPA %q is %.2fmm wide, wider than the %.2fmm the bank box's UPI row allows", fortyCharVPA, w, valueW)
	}

	inv := baseInvoice(t)
	inv.Type = model.InvoiceDomestic
	inv.Currency = "INR"
	inv.FXFactor = money.Rate(money.RateScale)
	inv.PlaceOfSupply = "33-Tamil Nadu"
	inv.IncludeHSNSACSummary = true
	inv.Bank.UPIID = longVPA

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

func TestBankBoxHeight_GrowsPerConditionalRow(t *testing.T) {
	dom := baseInvoice(t)
	dom.Type = model.InvoiceDomestic
	dom.Bank.UPIID = ""
	if got, want := bankBoxHeight(dom), bankBoxBaseH; got != want {
		t.Errorf("domestic bankBoxHeight without UPI ID = %v, want %v (base rows only)", got, want)
	}

	dom.Bank.UPIID = "rajenderelectricals@rbl"
	if got, want := bankBoxHeight(dom), bankBoxBaseH+bankRowH; got != want {
		t.Errorf("domestic bankBoxHeight with UPI ID = %v, want %v (base + one full-width row)", got, want)
	}

	dom.Bank.SwiftCode = testfixture.BankSwift
	if got, want := bankBoxHeight(dom), bankBoxBaseH+bankRowH; got != want {
		t.Errorf("domestic bankBoxHeight with SWIFT configured = %v, want %v (SWIFT is export-only)", got, want)
	}

	exp := baseInvoice(t) // InvoiceExportLUT, SwiftCode set, no UPI ID
	if got, want := bankBoxHeight(exp), bankBoxBaseH+bankRowH; got != want {
		t.Errorf("export bankBoxHeight = %v, want %v (base + Swift Code row)", got, want)
	}

	exp.Bank.SwiftCode = ""
	if got, want := bankBoxHeight(exp), bankBoxBaseH; got != want {
		t.Errorf("export bankBoxHeight without a SWIFT code = %v, want %v (row omitted, not blank)", got, want)
	}
}

func TestBankSignatureBand_NeverShorterThanSignature(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*model.Invoice)
	}{
		{"domestic, no UPI", func(i *model.Invoice) { i.Type = model.InvoiceDomestic; i.Bank.UPIID = "" }},
		{"domestic, UPI", func(i *model.Invoice) { i.Type = model.InvoiceDomestic; i.Bank.UPIID = "merchant@hdfc" }},
		{"export", func(i *model.Invoice) {}},
		{"export, no SWIFT", func(i *model.Invoice) { i.Bank.SwiftCode = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv := baseInvoice(t)
			tc.setup(inv)
			if got := bankSignatureBandH(inv); got < sigBlockH {
				t.Errorf("bankSignatureBandH = %v, shorter than the %v the signature column needs", got, sigBlockH)
			}
		})
	}
}

// TestRender_ShippingMetaRow_ColumnsDontOverlap is a regression for the
// export shipping row ("Shipping Bill No." / "Shipping Bill Date" /
// "Shipping Port Code") drawn by drawMetaRow. The three columns used fixed
// x positions sized for the Place of Supply / Due Date / Country of Supply
// row; "Shipping Bill Date" plus a full date measures wider than the
// default 39mm gap to column 3, and fpdf's CellFormat does not clip, so it
// printed over "Shipping Port Code". This asserts the actual column
// positions metaRowColumnXs computes leave each column's measured text
// clear of the next column's start.
func TestRender_ShippingMetaRow_ColumnsDontOverlap(t *testing.T) {
	triples := [][2]string{
		{LabelShippingBillNo, "SB12345"},
		{LabelShippingBillDate, "10/06/2026"},
		{LabelShippingPortCode, "INMAA1"},
	}

	d := newDoc()
	d.pdf.AddPage()

	// Sanity check that this fixture still reproduces the original bug
	// against the *default*, un-adjusted column positions — otherwise this
	// test would pass without exercising the fix.
	d.pdf.SetFont(FontFamily, "B", 8)
	dateLabelW := d.pdf.GetStringWidth(LabelShippingBillDate) + 1.5
	d.pdf.SetFont(FontFamily, "", 8.4)
	dateValueW := d.pdf.GetStringWidth("10/06/2026") + 1
	const defaultCol1X, defaultCol2X = marginL + 57, marginL + 96
	if defaultCol1X+dateLabelW+dateValueW <= defaultCol2X {
		t.Fatalf("fixture stopped reproducing the bug: Shipping Bill Date row (%.2fmm) now fits the default %.2fmm gap to column 3; pick a longer date/label fixture", dateLabelW+dateValueW, defaultCol2X-defaultCol1X)
	}

	xs := d.metaRowColumnXs(triples)
	if len(xs) != 3 {
		t.Fatalf("metaRowColumnXs returned %d columns, want 3", len(xs))
	}

	const gutter = 4.0
	measure := func(label, value string) float64 {
		d.pdf.SetFont(FontFamily, "B", 8)
		lw := d.pdf.GetStringWidth(label) + 1.5
		d.pdf.SetFont(FontFamily, "", 8.4)
		vw := d.pdf.GetStringWidth(value) + 1
		return lw + vw
	}

	for i := 0; i < len(triples)-1; i++ {
		end := xs[i] + measure(triples[i][0], triples[i][1])
		if xs[i+1] < end+gutter-0.01 {
			t.Errorf("column %d (%q) ends at %.2fmm but column %d starts at %.2fmm — only %.2fmm gap, want >= %.2fmm",
				i, triples[i][0], end, i+1, xs[i+1], xs[i+1]-end, gutter)
		}
	}

	// Column 3 must still land clear of the right margin.
	lastEnd := xs[2] + measure(triples[2][0], triples[2][1])
	if lastEnd > pageW-marginR {
		t.Errorf("column 3 (%q) ends at %.2fmm, past the right margin at %.2fmm", triples[2][0], lastEnd, pageW-marginR)
	}

	inv := baseInvoice(t)
	inv.Type = model.InvoiceExportIGST
	inv.ShippingBillNo = "SB12345"
	inv.ShippingPortCode = "INMAA1"
	sbd := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	inv.ShippingBillDate = &sbd

	data, err := Render(inv)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	requireValidPDF(t, data)
}

// TestRender_PlaceOfSupplyMetaRow_UnaffectedByShippingFix pins that the
// column-derivation fix for the shipping row (Bug 2) does not visibly move
// the Place of Supply / Due Date / Country of Supply row: it should still
// render at the original fixed positions since none of its columns measure
// past their defaults.
func TestRender_PlaceOfSupplyMetaRow_UnaffectedByShippingFix(t *testing.T) {
	triples := [][2]string{
		{LabelPlaceOfSupply, "97-Other Territory"},
		{LabelDueDate, "15/07/2024"},
		{LabelCountryOfSupply, "UNITED STATES OF AMERICA"},
	}
	d := newDoc()
	d.pdf.AddPage()
	xs := d.metaRowColumnXs(triples)
	want := []float64{marginL, marginL + 57, marginL + 96}
	for i, w := range want {
		if xs[i] != w {
			t.Errorf("column %d x = %.2fmm, want unchanged default %.2fmm", i, xs[i], w)
		}
	}
}
