package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/words"
)

// classic.go implements the monochrome, fully-ruled "Tally-style" GST tax
// invoice layout (model.LayoutClassic), reproduced from sample-domestic.jpeg.
// Every rule and glyph is drawn in black: this layout exists to print
// legibly on a black & white office printer, so nothing here may reach for
// the modern layout's red/yellow accent helpers (setAccentText, hline,
// colorHeaderFill). Instead this file keeps its own monoOuter/monoInner pen
// styles and classic-local drawing helpers, entirely independent of the
// shared grid/colour state the modern layout relies on.
//
// Layout, top to bottom (matches the numbered bands in sample-domestic.jpeg):
//  1. Title strip ("Tax INVOICE" + copy marker), outside the frame.
//  2. Header band: seller/consignee/buyer on the left, an invoice-meta grid
//     on the right, split by one vertical rule.
//  3. Items table, followed by unruled tax lines and a bold Total row.
//  4. Amount-in-words.
//  5. Optional HSN/SAC summary (+ UPI QR column).
//  6. Tax-amount-in-words.
//  7. Declaration + bank-details band.
//  8. Signature band.
//  9. Jurisdiction / computer-generated footer, outside the frame.
//
// Only the items table may split across a page; bands 4-8 are kept together
// via ensureSpace, mirroring the modern layout's footerBlockHeight pattern.

// ---- Monochrome pen styles ------------------------------------------------

const (
	classicLineOuter = 0.35 // heavier: outer frame edges, Total row rules
	classicLineInner = 0.15 // lighter: internal column/row dividers
)

// monoOuter selects the heavier black pen used for a band's outer frame.
func (d *doc) monoOuter() {
	d.pdf.SetDrawColor(0, 0, 0)
	d.pdf.SetLineWidth(classicLineOuter)
}

// monoInner selects the lighter black pen used for internal grid lines.
func (d *doc) monoInner() {
	d.pdf.SetDrawColor(0, 0, 0)
	d.pdf.SetLineWidth(classicLineInner)
}

// monoReset restores a neutral black hairline pen after a classic band
// finishes drawing. It never touches colorBorder/colorAccent — this layout
// must never depend on the modern layout's palette.
func (d *doc) monoReset() {
	d.pdf.SetDrawColor(0, 0, 0)
	d.pdf.SetLineWidth(0.2)
}

// classicSplitX is the vertical rule shared by every two-column band in this
// layout (header, declaration/bank, signature): 105mm left, 85mm right.
const classicSplitX = marginL + 105.0

const (
	classicLeftW  = 105.0
	classicRightW = pageW - marginR - classicSplitX // 85mm
)

// renderClassic draws the monochrome, fully-ruled GST tax-invoice layout
// (Tally-style) selected via model.LayoutClassic.
func (d *doc) renderClassic(inv *model.Invoice, result calc.InvoiceResult) {
	d.classicTitleStrip(inv)
	d.drawClassicHeaderBand(inv)

	// The footer block's height is measured before the table is drawn so the
	// table's cosmetic bottom padding can yield to it: on a short invoice the
	// padding is the only thing standing between a one-page invoice and a
	// footer pushed onto a second page.
	footerH := d.classicFooterBlockHeight(inv, result)
	d.classicDrawItemsTable(inv, result, footerH)

	// Bands 4-8 (words, HSN summary, tax words, declaration/bank,
	// signature) are kept together on one page, same as the modern
	// layout's footerBlockHeight gate.
	d.ensureSpace(footerH)

	d.classicAmountInWords(inv, result)
	d.drawClassicHSNSummary(inv, result)
	d.classicTaxAmountWords(result)
	d.drawClassicDeclarationBank(inv)
	d.drawClassicSignature(inv)
	d.drawClassicFooterLines(inv)
}

// ---- (1) Title strip -------------------------------------------------------

func (d *doc) classicTitleStrip(inv *model.Invoice) {
	y := d.pdf.GetY()
	d.pdf.SetFont(FontFamily, "B", 12)
	d.setNormalText()
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(contentW, 6, LabelClassicTitle, "", 0, "C", false, 0, "")

	d.pdf.SetFont(FontFamily, "B", 8.5)
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(contentW, 6, classicCopyLabel(inv.CopyType), "", 0, "R", false, 0, "")

	d.pdf.SetXY(marginL, y+6.5)
}

func classicCopyLabel(copyType string) string {
	switch strings.ToUpper(strings.TrimSpace(copyType)) {
	case LabelCopyDuplicate:
		return LabelClassicCopyDuplicate
	case LabelCopyTriplicate:
		return LabelClassicCopyTriplicate
	default:
		return LabelClassicCopyOriginal
	}
}

// ---- (2) Header band --------------------------------------------------------

// classicHeaderRow is one row of the right-hand invoice-meta grid: a
// left/right label+value pair, or (for the full-width "Terms of Delivery"
// row) just Label1/Value1.
type classicHeaderRow struct {
	Label1, Value1, Label2, Value2 string
}

// classicHeaderRowH must comfortably fit a 6.5pt caption stacked above an
// 8.5pt bold value (see classicCell): 8.0mm gives ~0.6mm of top/bottom
// breathing room around that stack, wide enough that the row's own bottom
// divider never strikes through the value text.
const classicHeaderRowH = 8.0
const classicHeaderMinLastRowH = 8.0

// drawClassicHeaderBand draws the seller/consignee/buyer block (left) and
// the invoice-meta grid (right), split by one vertical rule, boxed in a
// single outer frame. The two columns' natural content heights differ (the
// left grows with address line count, the right is a fixed-height grid), so
// the band height is the max of the two: the left column's last sub-block
// (Buyer/Bill-to) absorbs any slack below its own text, and the right
// column's last row (Terms of Delivery) stretches to reach the same bottom
// edge — both simply leave blank space above the shared bottom rule, which
// is drawn once, after both natural heights are known.
func (d *doc) drawClassicHeaderBand(inv *model.Invoice) float64 {
	y0 := d.pdf.GetY()

	posName, posCode := placeOfSupplyNameCode(inv.PlaceOfSupply)

	yA := d.classicSellerBlock(inv, y0)
	d.classicHDivider(marginL, classicSplitX, yA+0.6)
	yB := d.classicPartyBlock(yA+1.2, LabelClassicConsignee, inv.Customer.Name,
		inv.Customer.ShippingAddress, inv.Customer.GSTIN, posName, posCode)
	d.classicHDivider(marginL, classicSplitX, yB+0.6)
	yC := d.classicPartyBlock(yB+1.2, LabelClassicBuyer, inv.Customer.Name,
		inv.Customer.BillingAddress, inv.Customer.GSTIN, posName, posCode)

	rows := classicHeaderRightRows(inv)

	leftHeight := yC - y0
	rightHeight := float64(len(rows)-1)*classicHeaderRowH + classicHeaderMinLastRowH
	bandHeight := leftHeight
	if rightHeight > bandHeight {
		bandHeight = rightHeight
	}
	frameBottom := y0 + bandHeight

	d.drawClassicHeaderGrid(rows, y0, frameBottom)

	d.monoOuter()
	d.pdf.Rect(marginL, y0, contentW, bandHeight, "D")
	d.pdf.Line(classicSplitX, y0, classicSplitX, frameBottom)
	d.monoReset()

	d.pdf.SetXY(marginL, frameBottom)
	return frameBottom
}

// classicHDivider draws a thin horizontal rule spanning [x0,x1] at y.
func (d *doc) classicHDivider(x0, x1, y float64) {
	d.monoInner()
	d.pdf.Line(x0, y, x1, y)
	d.monoReset()
}

// classicSellerBlock draws the seller's logo/name/address/GSTIN/state block
// and returns the y position just below its last line.
func (d *doc) classicSellerBlock(inv *model.Invoice, y0 float64) float64 {
	pad := 2.0
	x := marginL + pad
	y := y0 + 1.2

	logoW := 0.0
	if inv.Seller.LogoPath != "" {
		if info, err := os.Stat(inv.Seller.LogoPath); err == nil && !info.IsDir() {
			if ext := strings.ToLower(filepath.Ext(inv.Seller.LogoPath)); ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				d.pdf.ImageOptions(inv.Seller.LogoPath, x, y, 16, 16, false, imageOptionsFor(ext), 0, "")
				logoW = 18
			}
		}
	}

	tx := x + logoW
	textW := classicLeftW - 2*pad - logoW
	yy := y

	d.pdf.SetFont(FontFamily, "B", 11)
	d.setNormalText()
	d.pdf.SetXY(tx, yy)
	d.pdf.CellFormat(textW, 4.6, inv.Seller.Name, "", 0, "L", false, 0, "")
	yy += 4.6

	d.pdf.SetFont(FontFamily, "", 8)
	for _, line := range inv.Seller.AddressLines {
		for _, wrapped := range d.wrapText(line, textW) {
			d.pdf.SetXY(tx, yy)
			d.pdf.CellFormat(textW, 3.6, wrapped, "", 0, "L", false, 0, "")
			yy += 3.6
		}
	}
	if inv.Seller.Phone != "" {
		d.pdf.SetXY(tx, yy)
		d.pdf.CellFormat(textW, 3.6, inv.Seller.Phone, "", 0, "L", false, 0, "")
		yy += 3.6
	}
	if inv.Seller.Email != "" {
		d.pdf.SetXY(tx, yy)
		d.pdf.CellFormat(textW, 3.6, inv.Seller.Email, "", 0, "L", false, 0, "")
		yy += 3.6
	}
	d.pdf.SetXY(tx, yy)
	d.pdf.CellFormat(textW, 3.6, "GSTIN/UIN: "+inv.Seller.GSTIN, "", 0, "L", false, 0, "")
	yy += 3.6
	d.pdf.SetXY(tx, yy)
	d.pdf.CellFormat(textW, 3.6, classicStateLine(inv.Seller.StateName, inv.Seller.StateCode), "", 0, "L", false, 0, "")
	yy += 3.6

	if logoBottom := y + 16 + 1; logoW > 0 && logoBottom > yy {
		yy = logoBottom
	}
	return yy
}

// classicPartyBlock draws one of the Consignee/Buyer sub-blocks (caption,
// name, wrapped address lines, GSTIN, state) starting at y0 and returns the
// y position just below its last line.
func (d *doc) classicPartyBlock(y0 float64, caption, name string, addr []string, gstin, stateName, stateCode string) float64 {
	pad := 2.0
	x := marginL + pad
	w := classicLeftW - 2*pad
	y := y0

	d.pdf.SetFont(FontFamily, "", 7)
	d.setNormalText()
	d.pdf.SetXY(x, y)
	d.pdf.CellFormat(w, 3.2, caption, "", 0, "L", false, 0, "")
	y += 3.4

	d.pdf.SetFont(FontFamily, "B", 9)
	for _, line := range d.wrapText(name, w) {
		d.pdf.SetXY(x, y)
		d.pdf.CellFormat(w, 4, line, "", 0, "L", false, 0, "")
		y += 4
	}

	d.pdf.SetFont(FontFamily, "", 8)
	for _, line := range addr {
		for _, wrapped := range d.wrapText(line, w) {
			d.pdf.SetXY(x, y)
			d.pdf.CellFormat(w, 3.6, wrapped, "", 0, "L", false, 0, "")
			y += 3.6
		}
	}

	d.pdf.SetXY(x, y)
	d.pdf.CellFormat(w, 3.6, "GSTIN/UIN : "+orDash(gstin), "", 0, "L", false, 0, "")
	y += 3.6

	d.pdf.SetXY(x, y)
	d.pdf.CellFormat(w, 3.6, classicStateLine(stateName, stateCode), "", 0, "L", false, 0, "")
	y += 3.6

	return y
}

// classicStateLine formats the "State Name / Code" line used in the classic
// layout's seller and party blocks. A bare two-digit value (e.g. "33") is
// treated as a GST state code, not a name — place of supply is often entered
// that way.
func classicStateLine(stateName, stateCode string) string {
	stateName = strings.TrimSpace(stateName)
	stateCode = strings.TrimSpace(stateCode)
	if stateCode == "" && isGSTStateCode(stateName) {
		stateCode = stateName
		stateName = ""
	}
	if stateName == "" && stateCode != "" {
		if resolved, ok := model.GSTStateName(stateCode); ok {
			stateName = resolved
		}
	}
	switch {
	case stateName != "" && stateCode != "":
		return fmt.Sprintf("State Name : %s, Code : %s", stateName, stateCode)
	case stateName != "":
		return "State Name : " + stateName
	case stateCode != "":
		return fmt.Sprintf("State Name : -, Code : %s", stateCode)
	default:
		return "State Name : -"
	}
}

// isGSTStateCode reports whether s is a bare two-digit GST state/UT code.
func isGSTStateCode(s string) bool {
	if len(s) != 2 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// placeOfSupplyNameCode splits a "33-Tamil Nadu"-formatted place of supply
// into its name and code, the inverse of model.Company.StateLabel. Used to
// derive the Consignee/Buyer "State Name" line, since neither Customer nor
// LineItem carries a separate recipient-state field.
func placeOfSupplyNameCode(pos string) (name, code string) {
	pos = strings.TrimSpace(pos)
	if i := strings.IndexByte(pos, '-'); i >= 0 {
		return strings.TrimSpace(pos[i+1:]), strings.TrimSpace(pos[:i])
	}
	if isGSTStateCode(pos) {
		if name, ok := model.GSTStateName(pos); ok {
			return name, pos
		}
		return "", pos
	}
	return pos, ""
}

// classicHeaderRightRows builds the 6 fixed two-column rows plus the
// full-width Terms of Delivery row for the header's right-hand grid. Export
// invoices swap in shipping-bill/port fields per the plan; domestic
// invoices leave those cells blank (which is itself part of the printed
// look — see drawClassicHeaderGrid).
func classicHeaderRightRows(inv *model.Invoice) []classicHeaderRow {
	dueVal := ""
	if !inv.DueDate.IsZero() {
		dueVal = "Due " + inv.DueDate.Format(dateLayout)
	}

	rows := []classicHeaderRow{
		{LabelClassicInvoiceNo, inv.Number, LabelClassicDated, inv.Date.Format(dateLayout)},
		{LabelClassicReferenceNoDate, inv.ReferenceNo, LabelClassicModeTerms, dueVal},
	}

	if !isExport(inv) {
		return append(rows, classicHeaderRow{LabelClassicTermsOfDeliver, "", "", ""})
	}

	return append(rows,
		classicHeaderRow{LabelShippingBillNo, inv.ShippingBillNo, LabelShippingBillDate, formatShippingBillDate(inv)},
		classicHeaderRow{LabelClassicPortCode, inv.ShippingPortCode, LabelClassicDestination, inv.CountryOfSupply},
		classicHeaderRow{LabelClassicTermsOfDeliver, fmt.Sprintf("Currency: %s @ %s", inv.Currency, inv.FXFactor.String()), "", ""},
	)
}

// drawClassicHeaderGrid draws the right-hand invoice-meta grid's text and
// internal rules. frameBottom (already known from the band-height
// computation in drawClassicHeaderBand) is where the Terms of Delivery
// row's cell ends, whether or not its own content reaches that far.
func (d *doc) drawClassicHeaderGrid(rows []classicHeaderRow, y0, frameBottom float64) {
	if len(rows) == 0 {
		return
	}
	x0 := classicSplitX
	w := classicRightW
	subW := w / 2

	paired := len(rows) - 1
	for i := 0; i < paired; i++ {
		ry := y0 + float64(i)*classicHeaderRowH
		d.classicCell(x0, ry, subW, classicHeaderRowH, rows[i].Label1, rows[i].Value1)
		d.classicCell(x0+subW, ry, subW, classicHeaderRowH, rows[i].Label2, rows[i].Value2)
	}
	lastY := y0 + float64(paired)*classicHeaderRowH
	last := rows[paired]
	d.classicCell(x0, lastY, w, frameBottom-lastY, last.Label1, last.Value1)

	d.monoInner()
	for i := 1; i <= paired; i++ {
		yy := y0 + float64(i)*classicHeaderRowH
		d.pdf.Line(x0, yy, x0+w, yy)
	}
	d.pdf.Line(x0+subW, y0, x0+subW, lastY)
	d.monoReset()
}

// classicCell draws a small caption above a bold value inside a labelled
// grid cell (no border — the caller draws the shared grid lines). A blank
// value is intentional and common in this layout (e.g. "Delivery Note"):
// the caption alone still prints, which is what makes the format
// recognisable.
func (d *doc) classicCell(x, y, w, h float64, caption, value string) {
	pad := 1.3
	d.pdf.SetFont(FontFamily, "", 6.5)
	d.setNormalText()
	d.pdf.SetXY(x+pad, y+0.7)
	d.pdf.CellFormat(w-2*pad, 3, caption, "", 0, "L", false, 0, "")
	if value == "" {
		return
	}
	d.pdf.SetFont(FontFamily, "B", 8.5)
	d.pdf.SetXY(x+pad, y+4.1)
	d.pdf.CellFormat(w-2*pad, 3.8, value, "", 0, "L", false, 0, "")
}

// ---- (3) Items table --------------------------------------------------------

// classicCol describes one column of the items table. Widths are fixed mm
// values (not proportional), summing to exactly contentW — see
// TestClassicItemColumns_SumToContentWidth.
type classicCol struct {
	ID     string
	Header string
	Width  float64
	Align  string
}

func classicItemColumns() []classicCol {
	return []classicCol{
		{"sl", ColClassicSlNo, 9, "C"},
		{"desc", ColClassicDesc, 84, "L"},
		{"hsn", ColClassicHSNSAC, 20, "C"},
		{"qty", ColClassicQty, 20, "R"},
		{"rate", ColClassicRate, 20, "R"},
		{"per", ColClassicPer, 10, "C"},
		{"amount", ColClassicAmount, 27, "R"},
	}
}

func classicColX(cols []classicCol, id string) float64 {
	x := marginL
	for _, c := range cols {
		if c.ID == id {
			return x
		}
		x += c.Width
	}
	return x
}

// classicMinBodyH is the height (header row to Total row) the items table
// body is padded to on short invoices — the large blank middle is part of
// the Tally look. It is cosmetic, so it yields to the footer: see the
// padding block in classicDrawItemsTable.
const classicMinBodyH = 55.0

// classicTotalsRowH is the height of the bold Total row closing the items
// table, and classicEOEH the "E. & O.E" line drawn under it.
const (
	classicTotalsRowH = tableLineH * 1.6
	classicEOEH       = 0.8 + 4.5
)

// classicDrawItemsTable renders the full items table: header, one row per
// line item, the unruled tax-summary lines, blank padding to reach
// classicMinBodyH, the bold Total row, and the trailing "E. & O.E" line.
//
// footerH is the measured height of the bands that follow (see
// classicFooterBlockHeight); the blank padding is trimmed to whatever room
// is left once that block is reserved.
func (d *doc) classicDrawItemsTable(inv *model.Invoice, result calc.InvoiceResult, footerH float64) {
	cols := classicItemColumns()
	d.classicItemsHeaderRow(cols)
	bodyTop := d.pdf.GetY()

	for i, lr := range result.Lines {
		li := inv.Items[i]
		values, rowH := d.classicItemRowLines(cols, i, li, lr, inv.FXFactor)
		if d.ensureSpace(rowH) {
			d.classicItemsHeaderRow(cols)
		}
		d.classicDrawItemRow(cols, values, rowH)
	}

	taxLines := classicTaxLines(result)
	if d.ensureSpace(float64(len(taxLines)) * tableLineH) {
		d.classicItemsHeaderRow(cols)
	}
	taxTop := d.pdf.GetY()
	descX := classicColX(cols, "desc")
	amountX := classicColX(cols, "amount")
	d.pdf.SetFont(FontFamily, "", 8)
	d.setNormalText()
	for _, tl := range taxLines {
		y := d.pdf.GetY()
		d.pdf.SetXY(descX, y)
		d.pdf.CellFormat(cols[1].Width, tableLineH, tl.Label, "", 0, "R", false, 0, "")
		d.pdf.SetXY(amountX, y)
		d.pdf.CellFormat(cols[6].Width, tableLineH, money.FormatINR(tl.Amount), "", 0, "R", false, 0, "")
		d.pdf.SetXY(marginL, y+tableLineH)
	}
	taxBottom := d.pdf.GetY()
	// No horizontal rules between the tax lines themselves (matches the
	// sample) — just the table's continuous left/right/column edges.
	d.classicVerticalEdges(cols, taxTop, taxBottom)

	// Pad the body to classicMinBodyH, but only within the room left on
	// this page once the Total row, the E. & O.E line and the whole footer
	// block are reserved. The padding is decoration; spending it would
	// push the footer onto a second page, which is the one thing this
	// single-sheet stationery format must not do.
	if padH := classicMinBodyH - (taxBottom - bodyTop); padH > 0 {
		_, pageHmm := d.pdf.GetPageSize()
		_, _, _, bm := d.pdf.GetMargins()
		reserved := classicTotalsRowH + classicEOEH + footerH
		if avail := pageHmm - bm - d.pdf.GetY() - reserved; avail < padH {
			padH = avail
		}
		if padH > 0 {
			padTop := d.pdf.GetY()
			d.classicVerticalEdges(cols, padTop, padTop+padH)
			d.pdf.SetXY(marginL, padTop+padH)
		}
	}

	if d.ensureSpace(classicTotalsRowH) {
		d.classicItemsHeaderRow(cols)
	}
	d.classicTotalsRow(cols, inv.Items, result)

	d.pdf.SetFont(FontFamily, "", 7.5)
	d.setNormalText()
	y := d.pdf.GetY() + 0.8
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(contentW, 4, LabelClassicEOE, "", 0, "R", false, 0, "")
	d.pdf.SetXY(marginL, y+4.5)
}

func (d *doc) classicItemsHeaderRow(cols []classicCol) {
	x0 := marginL
	y0 := d.pdf.GetY()
	h := 7.0

	d.pdf.SetFont(FontFamily, "B", 7.5)
	d.setNormalText()
	x := x0
	for _, c := range cols {
		d.drawHeaderCellText(c.Header, x, y0, c.Width, h)
		x += c.Width
	}
	d.classicRowGrid(cols, y0, h, false)
	d.pdf.SetXY(x0, y0+h)
}

// classicItemRowLines wraps one line item's cell text and returns the
// row height its wrapped description requires.
func (d *doc) classicItemRowLines(cols []classicCol, idx int, li model.LineItem, lr calc.LineResult, fx money.Rate) (map[string][]string, float64) {
	descLines := d.wrapText(li.Description, cols[1].Width-2)
	rowH := float64(len(descLines)) * tableLineH
	if rowH < tableLineH {
		rowH = tableLineH
	}
	values := map[string][]string{
		"sl":     {fmt.Sprintf("%d.", idx+1)},
		"desc":   descLines,
		"hsn":    {li.HSNSAC},
		"qty":    {classicQty(li.Quantity) + " " + li.Unit},
		"rate":   {money.FormatINR(money.ConvertUSDToINR(li.RateUSD, fx))},
		"per":    {li.Unit},
		"amount": {money.FormatINR(lr.TaxableINR)},
	}
	return values, rowH
}

func (d *doc) classicDrawItemRow(cols []classicCol, values map[string][]string, rowH float64) {
	y0 := d.pdf.GetY()
	d.pdf.SetFont(FontFamily, "", 8)
	d.setNormalText()
	x := marginL
	for _, c := range cols {
		yy := y0
		for _, ln := range values[c.ID] {
			d.pdf.SetXY(x, yy)
			d.pdf.CellFormat(c.Width, tableLineH, ln, "", 0, c.Align, false, 0, "")
			yy += tableLineH
		}
		x += c.Width
	}
	d.classicRowGrid(cols, y0, rowH, false)
	d.pdf.SetXY(marginL, y0+rowH)
}

// classicTaxLine is one line of the unruled tax summary printed under the
// item rows: an unlabelled taxable-value subtotal, then CGST+SGST or IGST,
// then CESS/Round Off when non-zero.
type classicTaxLine struct {
	Label  string
	Amount money.Amount
}

func classicTaxLines(result calc.InvoiceResult) []classicTaxLine {
	lines := []classicTaxLine{{"", result.TaxableINR}}
	switch result.Mode {
	case calc.TaxCGSTSGST:
		lines = append(lines, classicTaxLine{LabelClassicCentralGST, result.CGST})
		lines = append(lines, classicTaxLine{LabelClassicStateGST, result.SGST})
	default:
		lines = append(lines, classicTaxLine{LabelClassicIGST, result.IGST})
	}
	if result.CESS != 0 {
		lines = append(lines, classicTaxLine{LabelClassicCESS, result.CESS})
	}
	if result.RoundOff != 0 {
		lines = append(lines, classicTaxLine{LabelClassicRoundOff, result.RoundOff})
	}
	return lines
}

// classicVerticalEdges draws the table's continuous left/right outer edges
// and internal column dividers spanning [y0,y1], with no horizontal rules —
// used for the unruled tax-line and blank-padding regions of the table body.
func (d *doc) classicVerticalEdges(cols []classicCol, y0, y1 float64) {
	if y1 <= y0 {
		return
	}
	x := marginL
	d.monoOuter()
	d.pdf.Line(x, y0, x, y1)
	d.monoInner()
	xEnd := marginL + contentW
	for _, c := range cols {
		x += c.Width
		if x >= xEnd-0.001 {
			break
		}
		d.pdf.Line(x, y0, x, y1)
	}
	d.monoOuter()
	d.pdf.Line(xEnd, y0, xEnd, y1)
	d.monoReset()
}

// classicRowGrid draws one row's outer edges, internal column dividers, and
// bottom rule (heavier when outerBottom marks the table's true bottom edge,
// e.g. the header and Total rows).
func (d *doc) classicRowGrid(cols []classicCol, y0, h float64, outerBottom bool) {
	x := marginL
	xEnd := marginL + contentW

	d.monoOuter()
	d.pdf.Line(x, y0, x, y0+h)
	d.pdf.Line(xEnd, y0, xEnd, y0+h)

	d.monoInner()
	for _, c := range cols {
		x += c.Width
		if x >= xEnd-0.001 {
			break
		}
		d.pdf.Line(x, y0, x, y0+h)
	}

	if outerBottom {
		d.monoOuter()
	} else {
		d.monoInner()
	}
	d.pdf.Line(marginL, y0+h, xEnd, y0+h)
	d.monoReset()
}

// classicQtySummary sums the invoice's line-item quantities for the Total
// row. When every line shares the same unit the sum prints with it (e.g.
// "16.00 NOS"); otherwise just the bare summed number, since mixed units
// cannot be added meaningfully.
func classicQtySummary(items []model.LineItem) string {
	if len(items) == 0 {
		return ""
	}
	var sum money.Qty
	unit := items[0].Unit
	sameUnit := true
	for _, it := range items {
		sum += it.Quantity
		if it.Unit != unit {
			sameUnit = false
		}
	}
	if sameUnit && unit != "" {
		return classicQty(sum) + " " + unit
	}
	return classicQty(sum)
}

// classicQty formats a quantity the way this stationery format does: whole
// counts print bare ("2 NOS", "16 NOS" — as on the reference document), and
// only genuinely fractional quantities keep their decimals.
//
// money.Qty.String() always renders two decimals, which is right for the
// modern layout's numeric columns but reads as a unit price rather than a
// count against "per NOS" here.
func classicQty(q money.Qty) string {
	s := q.String()
	if strings.HasSuffix(s, ".00") {
		return strings.TrimSuffix(s, ".00")
	}
	// A single trailing zero on a fractional quantity is noise too: 2.50 -> 2.5.
	if strings.Contains(s, ".") && strings.HasSuffix(s, "0") {
		return strings.TrimSuffix(s, "0")
	}
	return s
}

// classicTotalsRow draws the bold Total row, ruled above (closing off the
// unruled tax-line/padding region) and below.
func (d *doc) classicTotalsRow(cols []classicCol, items []model.LineItem, result calc.InvoiceResult) {
	y0 := d.pdf.GetY()
	h := classicTotalsRowH

	d.monoOuter()
	d.pdf.Line(marginL, y0, marginL+contentW, y0)
	d.monoReset()

	d.pdf.SetFont(FontFamily, "B", 8.5)
	d.setNormalText()

	mergedW := cols[0].Width + cols[1].Width // Sl No. + Description
	d.pdf.SetXY(marginL, y0)
	d.pdf.CellFormat(mergedW, h, LabelTableTotal, "", 0, "R", false, 0, "")

	qtyX := classicColX(cols, "qty")
	d.pdf.SetXY(qtyX, y0)
	d.pdf.CellFormat(cols[3].Width, h, classicQtySummary(items), "", 0, "C", false, 0, "")

	amountX := classicColX(cols, "amount")
	d.pdf.SetXY(amountX, y0)
	d.pdf.CellFormat(cols[6].Width, h, "₹ "+money.FormatINR(result.GrandTotal), "", 0, "R", false, 0, "")

	d.classicRowGrid(cols, y0, h, true)
	d.pdf.SetXY(marginL, y0+h)
}

// ---- (4) Amount in words ----------------------------------------------------

func (d *doc) classicAmountInWords(inv *model.Invoice, result calc.InvoiceResult) {
	y := d.pdf.GetY() + 1.5
	d.pdf.SetFont(FontFamily, "", 7)
	d.setNormalText()
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(contentW, 3.2, LabelClassicAmountChargeable, "", 0, "L", false, 0, "")
	y += 3.6

	d.pdf.SetFont(FontFamily, "B", 9)
	for _, ln := range d.wrapText(words.INR(result.GrandTotal), contentW) {
		d.pdf.SetXY(marginL, y)
		d.pdf.CellFormat(contentW, 4.4, ln, "", 0, "L", false, 0, "")
		y += 4.4
	}
	if inv.Currency == "USD" {
		for _, ln := range d.wrapText(words.USD(result.TotalUSD), contentW) {
			d.pdf.SetXY(marginL, y)
			d.pdf.CellFormat(contentW, 4.4, ln, "", 0, "L", false, 0, "")
			y += 4.4
		}
	}
	d.pdf.SetXY(marginL, y+1)
}

// classicAmountInWordsHeight mirrors classicAmountInWords' vertical
// consumption so the footer keep-together estimate stays accurate.
func classicAmountInWordsHeight(inv *model.Invoice, result calc.InvoiceResult) float64 {
	h := 1.5 + 3.6 + 4.4
	if inv.Currency == "USD" {
		h += 4.4
	}
	return h + 1
}

// ---- (5) HSN/SAC summary ----------------------------------------------------

// HSN/SAC summary column widths for the classic layout. Two sets are
// defined: the full 190mm table (no UPI QR reserved), and the 156mm table
// used when a 34mm QR column is reserved on the left (34+156=190). Both
// mirror the proportions of the modern layout's hsn.go widths, just scaled
// to their own total — see TestClassicHSNColumns_SumToTargetWidth.
const (
	classicHSNCode190     = 28.0
	classicHSNTaxable190  = 32.0
	classicHSNRate190     = 16.0
	classicHSNAmount190   = 28.0
	classicHSNTotalTax190 = 42.0 // CGST/SGST: 28+32+16+28+16+28+42 = 190

	classicHSNIGSTTaxable190  = 42.0
	classicHSNIGSTRate190     = 24.0
	classicHSNIGSTAmount190   = 42.0
	classicHSNIGSTTotalTax190 = 54.0 // IGST: 28+42+24+42+54 = 190

	classicHSNCode156     = 23.0
	classicHSNTaxable156  = 26.0
	classicHSNRate156     = 13.0
	classicHSNAmount156   = 23.0
	classicHSNTotalTax156 = 35.0 // CGST/SGST: 23+26+13+23+13+23+35 = 156

	classicHSNIGSTTaxable156  = 34.0
	classicHSNIGSTRate156     = 20.0
	classicHSNIGSTAmount156   = 34.0
	classicHSNIGSTTotalTax156 = 45.0 // IGST: 23+34+20+34+45 = 156

	classicHSNQRColW = 34.0 // 34 + 156 = 190 = contentW
)

func buildClassicHSNColumns(mode calc.TaxMode, qr bool) []hsnCol {
	code := classicHSNCode190
	taxable, rate, amount, totalTax := classicHSNTaxable190, classicHSNRate190, classicHSNAmount190, classicHSNTotalTax190
	igstTaxable, igstRate, igstAmount, igstTotalTax := classicHSNIGSTTaxable190, classicHSNIGSTRate190, classicHSNIGSTAmount190, classicHSNIGSTTotalTax190
	if qr {
		code = classicHSNCode156
		taxable, rate, amount, totalTax = classicHSNTaxable156, classicHSNRate156, classicHSNAmount156, classicHSNTotalTax156
		igstTaxable, igstRate, igstAmount, igstTotalTax = classicHSNIGSTTaxable156, classicHSNIGSTRate156, classicHSNIGSTAmount156, classicHSNIGSTTotalTax156
	}

	if mode == calc.TaxCGSTSGST {
		return []hsnCol{
			{ID: "hsn", Header: ColHSNSummaryCode, Width: code, Align: "L"},
			{ID: "taxable", Header: ColHSNTaxableAmount, Width: taxable, Align: "R"},
			{ID: "cgst_rate", Header: ColHSNRate, Group: ColCGSTGroup, Width: rate, Align: "C"},
			{ID: "cgst_amt", Header: ColHSNAmount, Group: ColCGSTGroup, Width: amount, Align: "R"},
			{ID: "sgst_rate", Header: ColHSNRate, Group: ColSGSTGroup, Width: rate, Align: "C"},
			{ID: "sgst_amt", Header: ColHSNAmount, Group: ColSGSTGroup, Width: amount, Align: "R"},
			{ID: "total_tax", Header: ColHSNTotalTax, Width: totalTax, Align: "R"},
		}
	}
	return []hsnCol{
		{ID: "hsn", Header: ColHSNSummaryCode, Width: code, Align: "L"},
		{ID: "taxable", Header: ColHSNTaxableAmount, Width: igstTaxable, Align: "R"},
		{ID: "igst_rate", Header: ColHSNRate, Group: ColIGSTGroup, Width: igstRate, Align: "C"},
		{ID: "igst_amt", Header: ColHSNAmount, Group: ColIGSTGroup, Width: igstAmount, Align: "R"},
		{ID: "total_tax", Header: ColHSNTotalTax, Width: igstTotalTax, Align: "R"},
	}
}

// drawClassicHSNSummary renders the HSN/SAC-wise tax breakdown (domestic
// invoices with IncludeHSNSACSummary only — same gate as the modern
// layout), monochrome and unfilled, with a UPI QR column reserved on the
// left when showUPIQR is true.
func (d *doc) drawClassicHSNSummary(inv *model.Invoice, result calc.InvoiceResult) {
	if inv.Type != model.InvoiceDomestic || !inv.IncludeHSNSACSummary {
		return
	}
	rows := calc.HSNSummary(result)
	if len(rows) == 0 {
		return
	}

	qr := showUPIQR(inv, result)
	cols := buildClassicHSNColumns(result.Mode, qr)
	x0 := marginL
	if qr {
		x0 = marginL + classicHSNQRColW
	}

	need := hsnGap + hsnTitleH + classicHSNHeaderHeight(cols) + hsnRowH
	d.ensureSpace(need)

	y := d.pdf.GetY() + hsnGap
	d.pdf.SetFont(FontFamily, "B", 9)
	d.setNormalText()
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(contentW, hsnTitleH, LabelHSNSummary, "", 0, "L", false, 0, "")
	tableTop := y + hsnTitleH
	d.pdf.SetXY(x0, tableTop)

	d.classicHSNHeader(cols, x0)
	for _, row := range rows {
		if d.ensureSpace(hsnRowH) {
			d.classicHSNHeader(cols, x0)
		}
		d.classicHSNDataRow(cols, x0, row)
	}
	if d.ensureSpace(hsnRowH) {
		d.classicHSNHeader(cols, x0)
	}
	tableBottom := d.classicHSNTotalRow(cols, x0, rows)

	// The QR column's own content (image + "Scan to pay" caption) can be
	// taller than a short summary table (e.g. only 1-2 HSN rows): take the
	// max of the two so the caption never runs into whatever section is
	// drawn next.
	blockBottom := tableBottom
	if qr {
		qrSize := 30.0
		qrBlockBottom := tableTop + 2 + qrSize + 1 + 3.5
		if qrBlockBottom > blockBottom {
			blockBottom = qrBlockBottom
		}
		d.monoOuter()
		d.pdf.Rect(marginL, tableTop, classicHSNQRColW, blockBottom-tableTop, "D")
		d.monoReset()
		d.drawUPIQR(inv, result.GrandTotal, marginL+(classicHSNQRColW-qrSize)/2, tableTop+2, qrSize)
		d.pdf.SetFont(FontFamily, "", 7)
		d.setNormalText()
		d.pdf.SetXY(marginL, tableTop+2+qrSize+1)
		d.pdf.CellFormat(classicHSNQRColW, 3.5, LabelScanToPayUPI, "", 0, "C", false, 0, "")
	}

	d.pdf.SetXY(marginL, blockBottom+2)
}

func classicHSNHeaderHeight(cols []hsnCol) float64 {
	if hsnHasGroups(cols) {
		return hsnGroupH + hsnHeaderH
	}
	return hsnHeaderH
}

func (d *doc) classicHSNHeader(cols []hsnCol, x0 float64) {
	groups := hsnHasGroups(cols)
	h1 := 0.0
	if groups {
		h1 = hsnGroupH
	}
	h2 := hsnHeaderH
	totalH := h1 + h2
	y0 := d.pdf.GetY()

	d.pdf.SetFont(FontFamily, "B", 7.5)
	d.setNormalText()

	x := x0
	i := 0
	for i < len(cols) {
		c := cols[i]
		if groups && c.Group != "" {
			j := i + 1
			gw := c.Width
			for j < len(cols) && cols[j].Group == c.Group {
				gw += cols[j].Width
				j++
			}
			d.pdf.SetXY(x, y0)
			d.pdf.CellFormat(gw, h1, c.Group, "", 0, "C", false, 0, "")
			d.monoInner()
			d.pdf.Line(x, y0+h1, x+gw, y0+h1)
			sx := x
			for k := i; k < j; k++ {
				d.drawHeaderCellText(cols[k].Header, sx, y0+h1, cols[k].Width, h2)
				sx += cols[k].Width
			}
			x += gw
			i = j
			continue
		}
		d.drawHeaderCellText(c.Header, x, y0, c.Width, totalH)
		x += c.Width
		i++
	}

	xx := x0
	boundaries := []float64{xx}
	for _, c := range cols {
		xx += c.Width
		boundaries = append(boundaries, xx)
	}
	for b, bx := range boundaries {
		switch {
		case b == 0 || b == len(boundaries)-1:
			d.monoOuter()
			d.pdf.Line(bx, y0, bx, y0+totalH)
		case groups && cols[b-1].Group != "" && cols[b].Group == cols[b-1].Group:
			d.monoInner()
			d.pdf.Line(bx, y0+h1, bx, y0+totalH)
		default:
			d.monoInner()
			d.pdf.Line(bx, y0, bx, y0+totalH)
		}
	}
	d.monoOuter()
	d.pdf.Line(x0, y0, xx, y0)
	d.monoInner()
	d.pdf.Line(x0, y0+totalH, xx, y0+totalH)
	d.monoReset()

	d.pdf.SetXY(x0, y0+totalH)
}

func (d *doc) classicHSNDataRow(cols []hsnCol, x0 float64, row calc.HSNSummaryRow) {
	y0 := d.pdf.GetY()
	d.pdf.SetFont(FontFamily, "", 8)
	d.setNormalText()
	x := x0
	for _, c := range cols {
		d.pdf.SetXY(x, y0)
		d.pdf.CellFormat(c.Width, hsnRowH, hsnCellValue(c.ID, row), "", 0, c.Align, false, 0, "")
		x += c.Width
	}
	d.classicHSNRowGrid(cols, x0, y0, hsnRowH, false)
	d.pdf.SetXY(x0, y0+hsnRowH)
}

func (d *doc) classicHSNRowGrid(cols []hsnCol, x0, y0, h float64, outerBottom bool) {
	x := x0
	xEnd := x0 + hsnTableWidth(cols)

	d.monoOuter()
	d.pdf.Line(x, y0, x, y0+h)
	d.pdf.Line(xEnd, y0, xEnd, y0+h)

	d.monoInner()
	for _, c := range cols {
		x += c.Width
		if x >= xEnd-0.001 {
			break
		}
		d.pdf.Line(x, y0, x, y0+h)
	}

	if outerBottom {
		d.monoOuter()
	} else {
		d.monoInner()
	}
	d.pdf.Line(x0, y0+h, xEnd, y0+h)
	d.monoReset()
}

// classicHSNTotalRow draws the bold Total row and returns its bottom y.
func (d *doc) classicHSNTotalRow(cols []hsnCol, x0 float64, rows []calc.HSNSummaryRow) float64 {
	y0 := d.pdf.GetY()
	totals := sumHSNRows(rows)

	d.pdf.SetFont(FontFamily, "B", 8)
	d.setNormalText()
	x := x0
	for _, c := range cols {
		var text string
		switch c.ID {
		case "hsn":
			text = LabelTableTotal
		case "taxable":
			text = money.FormatINR(totals.TaxableINR)
		case "cgst_amt":
			text = money.FormatINR(totals.CGST)
		case "sgst_amt":
			text = money.FormatINR(totals.SGST)
		case "igst_amt":
			text = money.FormatINR(totals.IGST)
		case "total_tax":
			text = money.FormatINR(totals.TotalTax)
		}
		d.pdf.SetXY(x, y0)
		d.pdf.CellFormat(c.Width, hsnRowH, text, "", 0, c.Align, false, 0, "")
		x += c.Width
	}
	d.classicHSNRowGrid(cols, x0, y0, hsnRowH, true)
	bottom := y0 + hsnRowH
	d.pdf.SetXY(marginL, bottom)
	return bottom
}

// ---- (6) Tax amount in words ------------------------------------------------

func (d *doc) classicTaxAmountWords(result calc.InvoiceResult) {
	y := d.pdf.GetY() + 1.5
	d.pdf.SetFont(FontFamily, "", 8.5)
	d.setNormalText()
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(d.pdf.GetStringWidth(LabelClassicTaxAmountWords)+1, 4, LabelClassicTaxAmountWords, "", 0, "L", false, 0, "")
	labelW := d.pdf.GetStringWidth(LabelClassicTaxAmountWords) + 1

	d.pdf.SetFont(FontFamily, "B", 8.5)
	lines := d.wrapText(words.INR(result.TotalTax), contentW-labelW)
	vy := y
	for i, ln := range lines {
		x := marginL
		w := contentW
		if i == 0 {
			x = marginL + labelW
			w = contentW - labelW
		}
		d.pdf.SetXY(x, vy)
		d.pdf.CellFormat(w, 4, ln, "", 0, "L", false, 0, "")
		vy += 4
	}
	d.pdf.SetXY(marginL, vy+1)
}

func classicTaxAmountWordsHeight() float64 { return 1.5 + 4 + 1 }

// ---- (7) Declaration + bank details -----------------------------------------

const classicDeclBankMinH = 22.0

// drawClassicDeclarationBank draws the Declaration (left) and Company's
// Bank Details (right) band, split by the same vertical rule as the header
// band.
func (d *doc) drawClassicDeclarationBank(inv *model.Invoice) float64 {
	y0 := d.pdf.GetY()
	pad := 2.0

	declText := classicDeclaration(inv)
	// Wrap in the same font the lines are drawn in below, so the line count
	// here and in classicDeclarationBankHeight agree regardless of whatever
	// font the previous band happened to leave selected.
	d.pdf.SetFont(FontFamily, "", 7.5)
	declLines := d.wrapText(declText, classicLeftW-2*pad)
	leftH := 1.2 + 3.6 + float64(len(declLines))*3.6

	bankRows := classicBankRows(inv)
	rightH := 1.2 + 4.6 + float64(len(bankRows))*4.4

	bandHeight := leftH
	if rightH > bandHeight {
		bandHeight = rightH
	}
	if bandHeight < classicDeclBankMinH {
		bandHeight = classicDeclBankMinH
	}
	frameBottom := y0 + bandHeight

	ty := y0 + 1.2
	d.pdf.SetFont(FontFamily, "B", 7.5)
	d.setNormalText()
	d.pdf.SetXY(marginL+pad, ty)
	d.pdf.CellFormat(classicLeftW-2*pad, 3.4, LabelClassicDeclaration, "", 0, "L", false, 0, "")
	ty += 3.6
	d.pdf.SetFont(FontFamily, "", 7.5)
	for _, ln := range declLines {
		d.pdf.SetXY(marginL+pad, ty)
		d.pdf.CellFormat(classicLeftW-2*pad, 3.6, ln, "", 0, "L", false, 0, "")
		ty += 3.6
	}

	ry := y0 + 1.2
	d.pdf.SetFont(FontFamily, "B", 8.5)
	d.setNormalText()
	d.pdf.SetXY(classicSplitX+pad, ry)
	d.pdf.CellFormat(classicRightW-2*pad, 4, LabelClassicBankDetailsHeading, "", 0, "L", false, 0, "")
	ry += 4.6
	d.pdf.SetFont(FontFamily, "", 8)
	for _, r := range bankRows {
		d.pdf.SetXY(classicSplitX+pad, ry)
		d.pdf.CellFormat(classicRightW-2*pad, 4, r, "", 0, "L", false, 0, "")
		ry += 4.4
	}

	d.monoOuter()
	d.pdf.Rect(marginL, y0, contentW, bandHeight, "D")
	d.pdf.Line(classicSplitX, y0, classicSplitX, frameBottom)
	d.monoReset()

	d.pdf.SetXY(marginL, frameBottom)
	return frameBottom
}

// classicDeclaration returns the text for the Declaration box.
//
// Unlike the modern layout, where the declaration is simply omitted when
// empty, the classic format prints a labelled Declaration box as a permanent
// part of the form — the reference document carries one on a domestic invoice,
// where DeclarationDomestic is deliberately empty. Rather than print an empty
// captioned box, fall back to the standard wording that document uses.
//
// Settings → Declarations → Domestic supply still overrides this: whatever the
// user configures reaches here through declarationFor.
func classicDeclaration(inv *model.Invoice) string {
	if text := declarationFor(inv); text != "" {
		return text
	}
	return DeclarationClassicDefault
}

func classicBankRows(inv *model.Invoice) []string {
	rows := []string{
		fmt.Sprintf("%s : %s", LabelClassicAcHolderName, inv.Seller.Name),
		fmt.Sprintf("%s : %s", LabelClassicBankName, inv.Bank.BankName),
		fmt.Sprintf("%s : %s", LabelClassicAcNo, inv.Bank.AccountNumber),
		fmt.Sprintf("%s : %s & %s", LabelClassicBranchIFSC, inv.Bank.BranchName, inv.Bank.IFSC),
	}
	if showSwiftCode(inv) {
		rows = append(rows, fmt.Sprintf("%s : %s", LabelClassicSwift, inv.Bank.SwiftCode))
	}
	if showUPIID(inv) {
		rows = append(rows, fmt.Sprintf("%s : %s", LabelClassicUPIID, inv.Bank.UPIID))
	}
	return rows
}

// classicDeclarationBankHeight measures the band the same way
// drawClassicDeclarationBank lays it out.
//
// It is a method rather than a free function so it can wrap through
// d.wrapText: the declaration is typically one long sentence that wraps to
// two or three lines in a 105mm column, and counting only explicit newlines
// would under-measure it — leaving the keep-together estimate short and
// letting the footer run off the bottom of the page.
func (d *doc) classicDeclarationBankHeight(inv *model.Invoice) float64 {
	declLines := 0
	if text := classicDeclaration(inv); text != "" {
		d.pdf.SetFont(FontFamily, "", 7.5)
		declLines = len(d.wrapText(text, classicLeftW-4))
	}
	leftH := 1.2 + 3.6 + float64(declLines)*3.6
	rightH := 1.2 + 4.6 + float64(len(classicBankRows(inv)))*4.4
	h := leftH
	if rightH > h {
		h = rightH
	}
	if h < classicDeclBankMinH {
		h = classicDeclBankMinH
	}
	return h
}

// ---- (8) Signature band ------------------------------------------------------

const classicSignatureH = 26.0

func (d *doc) drawClassicSignature(inv *model.Invoice) float64 {
	y0 := d.pdf.GetY()
	pad := 2.0

	d.pdf.SetFont(FontFamily, "", 8.5)
	d.setNormalText()
	d.pdf.SetXY(marginL+pad, y0+classicSignatureH-5)
	d.pdf.CellFormat(classicLeftW-2*pad, 4, LabelClassicCustomerSeal, "", 0, "L", false, 0, "")

	d.pdf.SetFont(FontFamily, "", 9)
	d.pdf.SetXY(classicSplitX+pad, y0+1.5)
	d.pdf.CellFormat(classicRightW-2*pad, 4.5, fmt.Sprintf("%s %s", LabelFor, inv.Seller.Name), "", 0, "R", false, 0, "")

	if inv.Seller.SignaturePath != "" {
		if info, err := os.Stat(inv.Seller.SignaturePath); err == nil && !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(inv.Seller.SignaturePath))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				sigW := 32.0
				sigX := classicSplitX + classicRightW - pad - sigW
				d.pdf.ImageOptions(inv.Seller.SignaturePath, sigX, y0+7, sigW, 12, false, imageOptionsFor(ext), 0, "")
			}
		}
	}

	d.pdf.SetFont(FontFamily, "", 8)
	d.setNormalText()
	d.pdf.SetXY(classicSplitX+pad, y0+classicSignatureH-5)
	d.pdf.CellFormat(classicRightW-2*pad, 4, LabelAuthorisedSignatory, "", 0, "R", false, 0, "")

	frameBottom := y0 + classicSignatureH
	d.monoOuter()
	d.pdf.Rect(marginL, y0, contentW, classicSignatureH, "D")
	d.pdf.Line(classicSplitX, y0, classicSplitX, frameBottom)
	d.monoReset()

	d.pdf.SetXY(marginL, frameBottom)
	return frameBottom
}

// ---- (9) Below-frame jurisdiction footer ------------------------------------

func (d *doc) drawClassicFooterLines(inv *model.Invoice) {
	y := d.pdf.GetY() + 3
	d.pdf.SetFont(FontFamily, "", 7)
	d.setNormalText()
	if city := strings.TrimSpace(inv.Seller.City); city != "" {
		d.pdf.SetXY(marginL, y)
		d.pdf.CellFormat(contentW, 3.4, fmt.Sprintf(LabelClassicJurisdictionFmt, strings.ToUpper(city)), "", 0, "C", false, 0, "")
		y += 3.6
	}
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(contentW, 3.4, LabelClassicComputerGenerated, "", 0, "C", false, 0, "")
}

func classicFooterLinesHeight(inv *model.Invoice) float64 {
	h := 3.0 + 3.4
	if strings.TrimSpace(inv.Seller.City) != "" {
		h += 3.6
	}
	return h
}

// ---- Footer keep-together estimate ------------------------------------------

// classicFooterBlockHeight estimates the vertical space needed for bands
// 4-8 (amount words, optional HSN summary, tax words, declaration/bank,
// signature) so they can be kept together on one page, mirroring
// render.go's footerBlockHeight for the modern layout.
func (d *doc) classicFooterBlockHeight(inv *model.Invoice, result calc.InvoiceResult) float64 {
	h := classicAmountInWordsHeight(inv, result)
	if inv.Type == model.InvoiceDomestic && inv.IncludeHSNSACSummary {
		hsnH := hsnSummaryHeight(result)
		// A short summary table (few HSN rows) alongside a reserved UPI QR
		// column can be shorter than the QR image + "Scan to pay" caption
		// block itself — drawClassicHSNSummary already stretches the
		// section to fit the taller of the two, so the keep-together
		// estimate must account for that floor too.
		if showUPIQR(inv, result) {
			qrBlockH := hsnGap + hsnTitleH + 2 + 30 + 1 + 3.5 + 2
			if qrBlockH > hsnH {
				hsnH = qrBlockH
			}
		}
		h += hsnH
	}
	h += classicTaxAmountWordsHeight()
	h += d.classicDeclarationBankHeight(inv)
	h += classicSignatureH
	h += classicFooterLinesHeight(inv)
	return h
}
