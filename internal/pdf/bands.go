package pdf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-pdf/fpdf"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/upi"
)

const dateLayout = "02/01/2006"

func moneyINR(a money.Amount) string { return money.FormatINR(a) }

// ---- Title band -------------------------------------------------------

func (d *doc) drawTitleAndCopyBox(inv *model.Invoice) {
	y0 := d.pdf.GetY()

	d.pdf.SetFont(FontFamily, "B", 20)
	d.setAccentText()
	d.pdf.SetXY(marginL, y0)
	d.pdf.CellFormat(contentW, 10, LabelTitle, "", 0, "C", false, 0, "")

	// Dashed copy-type box, top right.
	boxW, boxH := 36.0, 14.0
	bx := pageW - marginR - boxW
	by := y0
	d.pdf.SetDrawColor(colorAccent[0], colorAccent[1], colorAccent[2])
	d.pdf.SetDashPattern([]float64{1, 1}, 0)
	d.pdf.Rect(bx, by, boxW, boxH, "D")
	d.pdf.SetDashPattern(nil, 0)
	d.pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])

	copyType := strings.ToUpper(strings.TrimSpace(inv.CopyType))
	if copyType == "" {
		copyType = LabelCopyOriginal
	}
	sub := copySubLabel(copyType)

	d.pdf.SetFont(FontFamily, "B", 11)
	d.setAccentText()
	d.pdf.SetXY(bx, by+3.2)
	d.pdf.CellFormat(boxW, 5, copyType, "", 0, "C", false, 0, "")
	d.pdf.SetFont(FontFamily, "", 8)
	d.setMutedText()
	d.pdf.SetXY(bx, by+8.5)
	d.pdf.CellFormat(boxW, 4, sub, "", 0, "C", false, 0, "")

	// Note: the dashed copy-type box (top right) is boxH=14mm tall starting
	// at this band's y0, so the gap below the title must stay >= 14mm on
	// that side or the box's dashed border will cross into the sender-meta
	// text that starts at this y. Kept at the original 13mm for that
	// reason — do not shrink further without also shrinking the box.
	d.setNormalText()
	d.pdf.SetXY(marginL, y0+13)
}

func copySubLabel(copyType string) string {
	switch copyType {
	case LabelCopyDuplicate:
		return LabelForTransporter
	case LabelCopyTriplicate:
		return LabelForSupplier
	default:
		return LabelForRecipient
	}
}

// ---- Sender block (left: logo/company) + Sender meta (right) ----------

func (d *doc) drawSenderBlock(inv *model.Invoice, y0 float64) float64 {
	x := marginL
	logoW := 0.0
	if inv.Seller.LogoPath != "" {
		if info, err := os.Stat(inv.Seller.LogoPath); err == nil && !info.IsDir() {
			if ext := strings.ToLower(filepath.Ext(inv.Seller.LogoPath)); ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				d.pdf.ImageOptions(inv.Seller.LogoPath, x, y0, 18, 18, false, imageOptionsFor(ext), 0, "")
				logoW = 21
			}
		}
	}

	textX := x + logoW
	y := y0
	d.pdf.SetFont(FontFamily, "B", 13)
	d.setAccentText()
	d.pdf.SetXY(textX, y)
	d.pdf.CellFormat(0, 5.5, inv.Seller.Name, "", 2, "L", false, 0, "")
	y += 5.5

	d.pdf.SetFont(FontFamily, "", 8)
	d.setNormalText()
	for _, line := range inv.Seller.AddressLines {
		d.pdf.SetXY(textX, y)
		d.pdf.CellFormat(90, 3.6, line, "", 0, "L", false, 0, "")
		y += 3.6
	}
	if inv.Seller.Phone != "" {
		d.pdf.SetXY(textX, y)
		d.pdf.CellFormat(90, 3.6, inv.Seller.Phone, "", 0, "L", false, 0, "")
		y += 3.6
	}
	if inv.Seller.Email != "" {
		d.pdf.SetXY(textX, y)
		d.pdf.CellFormat(90, 3.6, inv.Seller.Email, "", 0, "L", false, 0, "")
		y += 3.6
	}
	return y
}

func imageOptionsFor(ext string) fpdf.ImageOptions {
	tp := "PNG"
	if ext == ".jpg" || ext == ".jpeg" {
		tp = "JPG"
	}
	return fpdf.ImageOptions{ImageType: tp, ReadDpi: true}
}

func (d *doc) drawSenderMeta(inv *model.Invoice, y0 float64) float64 {
	rightX := marginL + 102
	labelColW := 14.0
	col2X := rightX + 46

	rows := [][2][2]string{
		{{LabelGSTIN, inv.Seller.GSTIN}, {LabelInvoiceDate, inv.Date.Format(dateLayout)}},
		{{LabelState, inv.Seller.StateLabel()}, {LabelInvoiceNo, inv.Number}},
		{{LabelPAN, inv.Seller.PAN}, {LabelReferenceNo, orDash(inv.ReferenceNo)}},
	}

	y := y0 + 1
	for _, row := range rows {
		d.pdf.SetFont(FontFamily, "B", 8)
		d.setMutedText()
		d.pdf.SetXY(rightX, y)
		d.pdf.CellFormat(labelColW, 4.2, row[0][0], "", 0, "L", false, 0, "")
		d.pdf.SetFont(FontFamily, "", 8)
		d.setNormalText()
		d.pdf.SetXY(rightX+labelColW, y)
		d.pdf.CellFormat(col2X-(rightX+labelColW)-2, 4.2, row[0][1], "", 0, "L", false, 0, "")

		d.pdf.SetFont(FontFamily, "B", 8)
		d.setMutedText()
		d.pdf.SetXY(col2X, y)
		d.pdf.CellFormat(22, 4.2, row[1][0], "", 0, "L", false, 0, "")
		d.pdf.SetFont(FontFamily, "", 8)
		d.setNormalText()
		d.pdf.SetXY(col2X+22, y)
		d.pdf.CellFormat(pageW-marginR-(col2X+22), 4.2, row[1][1], "", 0, "L", false, 0, "")

		y += 4.6
	}
	return y
}

// ---- Party row: Customer / Billing / Shipping --------------------------

// Party-row columns: customer | billing address | shipping address.
// Address cells wrap long lines so they cannot bleed into the next column
// (e.g. a pasted single-line address previously overran "Shipping Address").
func (d *doc) drawPartyRow(inv *model.Invoice) {
	y0 := d.pdf.GetY()
	col1X, col2X, col3X := marginL, marginL+62, marginL+128
	// Leave a 2mm gutter between columns so wrapped glyphs do not touch
	// the neighbouring label/value text.
	col1W := col2X - col1X - 2
	col2W := col3X - col2X - 2
	col3W := pageW - marginR - col3X

	d.sectionLabel(col1X, y0, LabelCustomerName)
	d.sectionLabel(col2X, y0, LabelBillingAddress)
	d.sectionLabel(col3X, y0, LabelShippingAddress)

	y := y0 + 4.5
	d.pdf.SetFont(FontFamily, "", 9)
	d.setNormalText()

	// Customer name (single line; wrap only if necessary).
	nameY := y
	for _, line := range d.wrapText(inv.Customer.Name, col1W) {
		d.pdf.SetXY(col1X, nameY)
		d.pdf.CellFormat(col1W, 4.2, line, "", 0, "L", false, 0, "")
		nameY += 4.2
	}

	by := y
	for _, line := range inv.Customer.BillingAddress {
		for _, wrapped := range d.wrapText(line, col2W) {
			d.pdf.SetXY(col2X, by)
			d.pdf.CellFormat(col2W, 4, wrapped, "", 0, "L", false, 0, "")
			by += 4
		}
	}
	sy := y
	for _, line := range inv.Customer.ShippingAddress {
		for _, wrapped := range d.wrapText(line, col3W) {
			d.pdf.SetXY(col3X, sy)
			d.pdf.CellFormat(col3W, 4, wrapped, "", 0, "L", false, 0, "")
			sy += 4
		}
	}

	// GSTIN sits under the customer column, below the name block (and at
	// least one address line's worth if the name was short).
	y1 := maxF(nameY, y+6)
	d.sectionLabel(col1X, y1, LabelCustomerGSTIN)
	y1 += 4.5
	d.pdf.SetFont(FontFamily, "", 9)
	d.setNormalText()
	d.pdf.SetXY(col1X, y1)
	d.pdf.CellFormat(col1W, 4.2, orDash(inv.Customer.GSTIN), "", 0, "L", false, 0, "")

	bottom := maxF(y1+5, maxF(by, sy)) + 2
	d.hline(bottom)
	d.pdf.SetXY(marginL, bottom+2)
}

func (d *doc) sectionLabel(x, y float64, text string) {
	d.pdf.SetFont(FontFamily, "B", 8)
	d.setMutedText()
	d.pdf.SetXY(x, y)
	d.pdf.CellFormat(60, 4, text, "", 0, "L", false, 0, "")
}

// ---- Meta rows (place of supply / shipping / currency) ----------------

func (d *doc) drawMetaRow(triples [][2]string) {
	y0 := d.pdf.GetY()
	xs := d.metaRowColumnXs(triples)
	for i, pair := range triples {
		if pair[0] == "" {
			continue
		}
		d.labelValue(xs[i], y0, pair[0], pair[1], 8, 8.4)
	}
	bottom := y0 + 5
	d.hline(bottom)
	d.pdf.SetXY(marginL, bottom+2)
}

// metaRowColumnXs computes the x start position of each column in a meta
// row (Place of Supply/Due Date/Country of Supply; the export shipping row;
// Currency/Conversion Factor — drawMetaRow is reused for all three).
//
// Default column start positions were sized against the widest row this
// layout carries at these default widths (Place of Supply / Due Date /
// Country of Supply): col3 gets a 94mm-wide slot, comfortably fitting
// "Country of Supply UNITED STATES OF AMERICA" at this font size without
// clipping.
//
// fpdf's CellFormat does not clip text to its cell width, so a fixed slot
// is only safe for the row it was measured against — the export shipping
// row ("Shipping Bill Date" + a full date, ~43mm) does not fit the default
// 39mm gap between col2 and col3 and used to print over "Shipping Port
// Code". Rather than hard-code a second set of positions for that row,
// columns are derived from the actual measured width of each label+value
// pair and only pushed right of the default when the previous column's
// real content needs more room; short rows (Place of Supply, Currency)
// never measure past their defaults, so they render at the same position
// as before this fix.
func (d *doc) metaRowColumnXs(triples [][2]string) []float64 {
	defaultXs := []float64{marginL, marginL + 57, marginL + 96}
	const gutter = 4.0 // minimum clear gap between a column's text and the next column's label

	xs := make([]float64, len(triples))
	x := defaultXs[0]
	for i, pair := range triples {
		if i < len(defaultXs) && defaultXs[i] > x {
			x = defaultXs[i]
		}
		xs[i] = x
		if pair[0] == "" {
			continue
		}
		d.pdf.SetFont(FontFamily, "B", 8)
		labelW := d.pdf.GetStringWidth(pair[0]) + 1.5
		d.pdf.SetFont(FontFamily, "", 8.4)
		valueW := d.pdf.GetStringWidth(pair[1]) + 1
		x += labelW + valueW + gutter
	}
	return xs
}

// ---- Summary ------------------------------------------------------------

func (d *doc) drawSummary(result calc.InvoiceResult) {
	y := d.pdf.GetY() + 1
	x := pageW - marginR - 90

	row := func(label, value string, size float64, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		d.pdf.SetFont(FontFamily, style, size)
		d.setNormalText()
		d.pdf.SetXY(x, y)
		d.pdf.CellFormat(50, size/1.8, label, "", 0, "R", false, 0, "")
		d.pdf.SetXY(x+50, y)
		d.pdf.CellFormat(40, size/1.8, value, "", 0, "R", false, 0, "")
		y += size/1.8 + 1
	}

	row(LabelTaxableAmount, "₹ "+moneyINR(result.TaxableINR), 8.5, false)
	row(LabelTotalTax, "₹ "+moneyINR(result.TotalTax), 8.5, false)
	if result.RoundOff != 0 {
		row(LabelRoundOff, "₹ "+moneyINR(result.RoundOff), 8.5, false)
	}
	y += 0.5
	d.pdf.SetDrawColor(colorAccent[0], colorAccent[1], colorAccent[2])
	d.pdf.Line(x, y, x+90, y)
	d.pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	y += 1.5
	row(LabelTotalValue, "₹ "+moneyINR(result.GrandTotal), 12.5, true)

	d.pdf.SetXY(marginL, y+2)
}

// ---- Words block ---------------------------------------------------------

func (d *doc) drawWordsBlock(inv *model.Invoice, inrWords, usdWords string) {
	y := d.pdf.GetY()
	labelW := 60.0
	valueX := pageW - marginR - 100
	if valueX < marginL+labelW {
		valueX = marginL + labelW
	}
	labelX := valueX - labelW - 2

	line := func(label, value string) {
		d.pdf.SetFont(FontFamily, "B", 8)
		d.setMutedText()
		d.pdf.SetXY(labelX, y)
		d.pdf.CellFormat(labelW, 4, label, "", 0, "L", false, 0, "")
		d.pdf.SetFont(FontFamily, "", 8)
		d.setNormalText()
		vw := pageW - marginR - valueX
		lines := d.wrapText(value, vw)
		d.pdf.SetXY(valueX, y)
		for _, ln := range lines {
			d.pdf.SetXY(valueX, y)
			d.pdf.CellFormat(vw, 4, ln, "", 0, "L", false, 0, "")
			y += 4
		}
	}

	line(LabelAmountInWordsINR, inrWords)
	if inv.Currency == "USD" {
		line(LabelAmountInWordsUSD, usdWords)
	}
	d.pdf.SetXY(marginL, y+2)
}

// ---- Bank details + signature -------------------------------------------

func (d *doc) drawBankAndSignature(inv *model.Invoice, result calc.InvoiceResult) {
	y0 := d.pdf.GetY() + 1.5
	boxW, boxH := 110.0, bankBoxHeight(inv)
	// The signature column keeps its own height: on a domestic invoice with
	// no UPI ID the bank box is only two rows tall, and hanging the
	// signature rule off boxH would drag it through the signature image.
	bandH := bankSignatureBandH(inv)
	sigX := pageW - marginR - 70
	qrSize := 38.0

	hasUPI := showUPIQR(inv, result)

	d.pdf.SetDrawColor(colorAccent[0], colorAccent[1], colorAccent[2])
	d.pdf.SetLineWidth(0.3)
	d.pdf.Rect(marginL, y0, boxW, boxH, "D")
	d.pdf.SetLineWidth(0.2)
	d.pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])

	pad := 3.0
	d.pdf.SetFont(FontFamily, "B", 9)
	d.setNormalText()
	d.pdf.SetXY(marginL+pad, y0+pad)
	d.pdf.CellFormat(boxW-2*pad, 4.5, LabelBankDetails, "", 0, "L", false, 0, "")

	rowY := y0 + pad + 5.5
	// labelW/valueW fit every two-column value here except the UPI VPA: a
	// realistic VPA ("rajenderelectricals@rbl" at 8pt) measures ~30.7mm,
	// wider than this 25mm cell, and fpdf's CellFormat does not clip — it
	// would print straight over the "Swift Code:" label that used to sit at
	// x2. SWIFT/BIC codes are always <=11 chars and never overflow, so only
	// the UPI ID needs different treatment: it gets its own full-width row
	// below (see upiRow) instead of sharing this two-column layout.
	labelW, valueW := 30.0, 25.0
	x1 := marginL + pad
	x2 := x1 + labelW + valueW + 4

	bankRow := func(l1, v1, l2, v2 string) {
		d.pdf.SetFont(FontFamily, "B", 8)
		d.setMutedText()
		d.pdf.SetXY(x1, rowY)
		d.pdf.CellFormat(labelW, 4.2, l1, "", 0, "L", false, 0, "")
		d.pdf.SetFont(FontFamily, "", 8)
		d.setNormalText()
		d.pdf.SetXY(x1+labelW, rowY)
		d.pdf.CellFormat(valueW, 4.2, v1, "", 0, "L", false, 0, "")
		if l2 != "" {
			d.pdf.SetFont(FontFamily, "B", 8)
			d.setMutedText()
			d.pdf.SetXY(x2, rowY)
			d.pdf.CellFormat(24, 4.2, l2, "", 0, "L", false, 0, "")
			d.pdf.SetFont(FontFamily, "", 8)
			d.setNormalText()
			d.pdf.SetXY(x2+24, rowY)
			d.pdf.CellFormat(boxW-pad-(x2+24-marginL), 4.2, v2, "", 0, "L", false, 0, "")
		}
		rowY += 5.4
	}

	// upiRow prints a label+value pair spanning the box's full inner width
	// (~74mm at these margins), used only for the UPI ID so a long VPA has
	// room to grow without a neighbouring label to collide with.
	upiRow := func(label, value string) {
		d.pdf.SetFont(FontFamily, "B", 8)
		d.setMutedText()
		d.pdf.SetXY(x1, rowY)
		d.pdf.CellFormat(labelW, 4.2, label, "", 0, "L", false, 0, "")
		d.pdf.SetFont(FontFamily, "", 8)
		d.setNormalText()
		d.pdf.SetXY(x1+labelW, rowY)
		d.pdf.CellFormat(boxW-pad-(x1+labelW-marginL), 4.2, value, "", 0, "L", false, 0, "")
		rowY += 5.4
	}

	bankRow(LabelAccountNumber, inv.Bank.AccountNumber, LabelIFSC, inv.Bank.IFSC)
	bankRow(LabelBankName, inv.Bank.BankName, LabelBranchName, inv.Bank.BranchName)
	if showSwiftCode(inv) {
		bankRow(LabelSwiftCode, inv.Bank.SwiftCode, "", "")
	}
	if showUPIID(inv) {
		upiRow(LabelUPIID, inv.Bank.UPIID)
	}

	// Signature block, right-aligned beside the bank box.
	sigY := y0
	d.pdf.SetFont(FontFamily, "", 9)
	d.setNormalText()
	d.pdf.SetXY(sigX, sigY)
	d.pdf.CellFormat(70, 4.5, fmt.Sprintf("%s %s", LabelFor, inv.Seller.Name), "", 0, "R", false, 0, "")

	if inv.Seller.SignaturePath != "" {
		if info, err := os.Stat(inv.Seller.SignaturePath); err == nil && !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(inv.Seller.SignaturePath))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				d.pdf.ImageOptions(inv.Seller.SignaturePath, sigX+30, sigY+6, 30, 12, false, imageOptionsFor(ext), 0, "")
			}
		}
	}

	lineY := y0 + bandH - 8
	d.pdf.SetDrawColor(colorText[0], colorText[1], colorText[2])
	d.pdf.Line(sigX+20, lineY, pageW-marginR, lineY)
	d.pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	d.pdf.SetFont(FontFamily, "", 8)
	d.setMutedText()
	d.pdf.SetXY(sigX, lineY+1.5)
	d.pdf.CellFormat(70, 4, LabelAuthorisedSignatory, "", 0, "R", false, 0, "")

	blockBottom := y0 + bandH
	if hasUPI {
		qrY := y0 + boxH + 2
		d.drawUPIQR(inv, result.GrandTotal, marginL, qrY, qrSize)
		d.pdf.SetFont(FontFamily, "", 7)
		d.setMutedText()
		d.pdf.SetXY(marginL+qrSize+2, qrY+qrSize/2-1.5)
		d.pdf.CellFormat(boxW-qrSize-2, 3, LabelScanToPayUPI, "", 0, "L", false, 0, "")
		blockBottom = maxF(blockBottom, qrY+qrSize+2)
	}

	d.pdf.SetXY(marginL, blockBottom+3)
}

func (d *doc) drawUPIQR(inv *model.Invoice, grandTotal money.Amount, x, y, size float64) {
	uri := upi.PaymentURI(inv.Bank.UPIID, grandTotal, inv.Number)
	png, err := upi.PNG(uri, 512)
	if err != nil {
		return
	}
	name := "upi-qr-" + inv.Number
	opts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
	d.pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(png))
	d.pdf.ImageOptions(name, x, y, size, size, false, opts, 0, "")
}

// ---- Declaration footer ---------------------------------------------------

func (d *doc) drawDeclaration(text string) {
	if text == "" {
		return
	}
	y := d.pdf.GetY() + 2
	d.pdf.SetFont(FontFamily, "B", 7.5)
	d.setNormalText()
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(contentW, 4, text, "", 0, "L", false, 0, "")
	d.pdf.SetXY(marginL, y+5)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
