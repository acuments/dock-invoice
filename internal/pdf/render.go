package pdf

import (
	"bytes"
	"fmt"
	"strings"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/words"
)

// Render draws the full invoice (title through footer declaration) and
// returns the resulting PDF as bytes. inv must already carry its seller and
// bank snapshot (model.Invoice.Seller / .Bank) — Render performs no
// database or settings lookups of its own.
func Render(inv *model.Invoice) ([]byte, error) {
	if inv == nil {
		return nil, fmt.Errorf("pdf: nil invoice")
	}

	result := calc.ComputeInvoice(inv)

	d := newDoc()
	d.pdf.AddPage()

	switch model.NormalizeLayoutStyle(inv.LayoutStyle) {
	case model.LayoutClassic:
		d.renderClassic(inv, result)
	default:
		d.renderModern(inv, result)
	}

	var buf bytes.Buffer
	if err := d.pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf: render: %w", err)
	}
	return buf.Bytes(), nil
}

// renderModern draws the default accent-coloured layout: title through
// footer declaration, plus the opt-in HSN/SAC summary. This is the band
// sequence Render always ran before LayoutStyle existed.
func (d *doc) renderModern(inv *model.Invoice, result calc.InvoiceResult) {
	d.drawTitleAndCopyBox(inv)

	y0 := d.pdf.GetY()
	leftBottom := d.drawSenderBlock(inv, y0)
	rightBottom := d.drawSenderMeta(inv, y0)
	y := maxF(leftBottom, rightBottom) + 2
	d.hline(y)
	d.pdf.SetXY(marginL, y+2)

	d.drawPartyRow(inv)

	supplyRow := [][2]string{
		{LabelPlaceOfSupply, inv.PlaceOfSupply},
		{LabelDueDate, formatMaybeDate(inv.DueDate)},
	}
	if inv.Type != model.InvoiceDomestic {
		supplyRow = append(supplyRow, [2]string{LabelCountryOfSupply, inv.CountryOfSupply})
	}
	d.drawMetaRow(supplyRow)

	if isExport(inv) {
		d.drawMetaRow([][2]string{
			{LabelShippingBillNo, orDash(inv.ShippingBillNo)},
			{LabelShippingBillDate, formatShippingBillDate(inv)},
			{LabelShippingPortCode, orDash(inv.ShippingPortCode)},
		})
	}

	if inv.Type != model.InvoiceDomestic {
		d.drawMetaRow([][2]string{
			{LabelCurrency, inv.Currency},
			{LabelConversionFactor, inv.FXFactor.String()},
			{"", ""},
		})
	}

	d.drawItemsTable(inv, result)

	// Keep the summary/words/bank/signature/declaration block together: if
	// it can't all fit on the remaining space of this page, start a fresh
	// page for it rather than splitting it. The optional HSN summary sits
	// after that block and page-breaks independently.
	d.ensureSpace(footerBlockHeight(inv, result))

	d.drawSummary(result)

	inrWords := words.INR(result.GrandTotal)
	usdWords := ""
	if inv.Currency == "USD" {
		usdWords = words.USD(result.TotalUSD)
	}
	d.drawWordsBlock(inv, inrWords, usdWords)

	d.drawBankAndSignature(inv, result)

	d.drawDeclaration(declarationFor(inv))

	// HSN/SAC summary is opt-in (Settings → IncludeHSNSACSummary), domestic
	// only, and always at the foot of the invoice (below signature /
	// declaration).
	if inv.Type == model.InvoiceDomestic && inv.IncludeHSNSACSummary {
		d.drawHSNSummary(result)
	}
}

// footerBlockHeight estimates the vertical space needed for the
// summary+words+bank/signature+declaration block, so it can be kept intact
// on one page. HSN summary height is intentionally excluded — that section
// is drawn after bank/signature when enabled.
func footerBlockHeight(inv *model.Invoice, result calc.InvoiceResult) float64 {
	h := 4.7*2 + 2 // taxable + tax rows
	if result.RoundOff != 0 {
		h += 4.7
	}
	h += 2 + 7.2 // divider + total value row
	h += 4 + 2   // INR words
	if inv.Currency == "USD" {
		h += 4
	}
	h += bankSignatureBlockHeight(inv, result)
	if declarationFor(inv) != "" {
		h += 9
	}
	return h
}

func declarationFor(inv *model.Invoice) string {
	if inv.Notes != "" {
		return inv.Notes
	}
	switch inv.Type {
	case model.InvoiceExportLUT:
		return DeclarationLUT
	case model.InvoiceExportIGST:
		return DeclarationIGST
	default:
		return DeclarationDomestic
	}
}

func formatMaybeDate(t interface{ Format(string) string }) string {
	return t.Format(dateLayout)
}

// isExport reports whether inv uses one of the two export GST treatments.
func isExport(inv *model.Invoice) bool {
	return inv.Type == model.InvoiceExportLUT || inv.Type == model.InvoiceExportIGST
}

// showUPIID reports whether the UPI ID belongs on this invoice at all. UPI is
// an India-only rail, so it is printed for domestic supplies only — an export
// buyer cannot pay a VPA, and showing one there is misleading.
func showUPIID(inv *model.Invoice) bool {
	return inv.Type == model.InvoiceDomestic && strings.TrimSpace(inv.Bank.UPIID) != ""
}

func showSwiftCode(inv *model.Invoice) bool {
	return isExport(inv) && strings.TrimSpace(inv.Bank.SwiftCode) != ""
}

// showUPIQR reports whether the scan-to-pay QR block is drawn. It needs a UPI
// ID that qualifies for this invoice plus a positive amount to encode.
func showUPIQR(inv *model.Invoice, result calc.InvoiceResult) bool {
	return showUPIID(inv) && result.GrandTotal > 0
}

const (
	bankRowH     = 5.4
	bankBoxBaseH = 20.6
)

const sigBlockH = 26.0

func bankBoxHeight(inv *model.Invoice) float64 {
	boxH := bankBoxBaseH
	if showSwiftCode(inv) {
		boxH += bankRowH
	}
	if showUPIID(inv) {
		boxH += bankRowH
	}
	return boxH
}

func bankSignatureBandH(inv *model.Invoice) float64 {
	return maxF(bankBoxHeight(inv), sigBlockH)
}

func bankSignatureBlockHeight(inv *model.Invoice, result calc.InvoiceResult) float64 {
	bottom := bankSignatureBandH(inv)
	if showUPIQR(inv, result) {
		if qrBottom := bankBoxHeight(inv) + 2 + 38 + 2; qrBottom > bottom {
			bottom = qrBottom
		}
	}
	return 1.5 + bottom + 3
}

func formatShippingBillDate(inv *model.Invoice) string {
	if inv.ShippingBillDate == nil {
		return "-"
	}
	return inv.ShippingBillDate.Format(dateLayout)
}
