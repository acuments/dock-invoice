package pdf

import (
	"fmt"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

// tableColumn describes one column of the line-item table. The table
// renderer is driven entirely by a []tableColumn built for the invoice's tax
// mode and currency, so the IGST-vs-CGST/SGST and with/without-USD variants
// share this single rendering code path (per the plan's requirement that the
// table not be a fixed layout).
type tableColumn struct {
	ID     string
	Header string // header text; "\n" splits it onto a second line
	Group  string // non-empty: merged with adjacent columns sharing the
	// same Group under one spanning header row (only "Total" uses this,
	// spanning the INR/USD sub-columns on export invoices)
	Width float64
	Align string // "L", "C" or "R"
}

const (
	colFixedItem     = 44.0
	colFixedHSN      = 15.0
	colFixedQty      = 13.0
	colFixedRate     = 19.0
	colFixedDiscount = 15.0
	colFixedTaxable  = 19.0
)

// buildColumns returns the column set for the invoice's tax mode and
// currency (USD column present only for export invoices).
func buildColumns(mode calc.TaxMode, showUSD bool) []tableColumn {
	cols := []tableColumn{
		{ID: "item", Header: ColItem, Width: colFixedItem, Align: "L"},
		{ID: "hsn", Header: ColHSNSAC, Width: colFixedHSN, Align: "C"},
		{ID: "qty", Header: ColQuantity, Width: colFixedQty, Align: "C"},
		{ID: "rate", Header: ColRatePerItem, Width: colFixedRate, Align: "R"},
		{ID: "discount", Header: ColDiscount, Width: colFixedDiscount, Align: "R"},
		{ID: "taxable", Header: ColTaxableValue, Width: colFixedTaxable, Align: "R"},
	}

	switch {
	case showUSD: // export types: single IGST col + CESS + Total{INR,USD}
		cols = append(cols,
			tableColumn{ID: "igst", Header: ColIGST, Width: 15, Align: "C"},
			tableColumn{ID: "cess", Header: ColCESS, Width: 13, Align: "R"},
			tableColumn{ID: "total_inr", Header: ColTotalINR, Group: ColTotalGroup, Width: 19, Align: "R"},
			tableColumn{ID: "total_usd", Header: ColTotalUSD, Group: ColTotalGroup, Width: 18, Align: "R"},
		)
	case mode == calc.TaxCGSTSGST: // domestic intra-state
		cols = append(cols,
			tableColumn{ID: "cgst", Header: ColCGST, Width: 13, Align: "C"},
			tableColumn{ID: "sgst", Header: ColSGST, Width: 13, Align: "C"},
			tableColumn{ID: "cess", Header: ColCESS, Width: 13, Align: "R"},
			tableColumn{ID: "total_inr", Header: ColTotalINROnly, Width: 26, Align: "R"},
		)
	default: // domestic inter-state
		cols = append(cols,
			tableColumn{ID: "igst", Header: ColIGST, Width: 20, Align: "C"},
			tableColumn{ID: "cess", Header: ColCESS, Width: 13, Align: "R"},
			tableColumn{ID: "total_inr", Header: ColTotalINROnly, Width: 32, Align: "R"},
		)
	}
	return cols
}

const (
	tableLineH   = 4.0
	headerRowH   = 6.0
	headerGroupH = 4.5
)

// drawItemsTable renders the full line-item table, including the shared
// pale-yellow header (repeated on every page the table spans) and the
// totals row. Rows wrap and grow in height to fit description/HSN text.
func (d *doc) drawItemsTable(inv *model.Invoice, result calc.InvoiceResult) {
	showUSD := inv.Currency == "USD"
	cols := buildColumns(result.Mode, showUSD)

	d.drawTableHeader(cols)

	for i, lr := range result.Lines {
		li := inv.Items[i]
		lines := make([][]string, len(cols))
		maxLines := 1
		for ci, c := range cols {
			lines[ci] = d.cellLines(c, i, lr, li, inv.FXFactor)
			if len(lines[ci]) > maxLines {
				maxLines = len(lines[ci])
			}
		}
		rowH := float64(maxLines) * tableLineH

		// Keep row content atomic: if it doesn't fit on the current page,
		// start a new page and repeat the table header there.
		if d.ensureSpace(rowH) {
			d.drawTableHeader(cols)
		}

		d.drawTableRow(cols, lines, rowH)
	}

	// Keep the totals row with the table: break before it rather than
	// splitting it across pages.
	if d.ensureSpace(tableLineH) {
		d.drawTableHeader(cols)
	}
	d.drawTotalsRow(cols, result)
}

// drawTableRow renders one line-item row given its pre-wrapped per-column
// text lines and overall row height.
func (d *doc) drawTableRow(cols []tableColumn, lines [][]string, rowH float64) {
	x0 := marginL
	y0 := d.pdf.GetY()

	d.pdf.SetFont(FontFamily, "", 8)
	d.setNormalText()

	x := x0
	for ci, c := range cols {
		yy := y0
		for _, ln := range lines[ci] {
			d.pdf.SetXY(x, yy)
			d.pdf.CellFormat(c.Width, tableLineH, ln, "", 0, c.Align, false, 0, "")
			yy += tableLineH
		}
		x += c.Width
	}

	d.drawRowGrid(cols, y0, rowH, false)
	d.pdf.SetXY(x0, y0+rowH)
}

// drawRowGrid draws the vertical column separators and bottom border for a
// table row spanning height h starting at y0. The left/right table edges
// are always drawn heavier (outer frame); internal column dividers and the
// row's bottom line are drawn light, except outerBottom (used for the
// table's final totals row) which draws the bottom line heavier too, since
// that is the true bottom edge of the table.
func (d *doc) drawRowGrid(cols []tableColumn, y0, h float64, outerBottom bool) {
	x := marginL
	xEnd := marginL + tableWidth(cols)

	d.setGridOuter()
	d.pdf.Line(x, y0, x, y0+h)
	d.pdf.Line(xEnd, y0, xEnd, y0+h)

	d.setGridInner()
	for _, c := range cols {
		x += c.Width
		if x >= xEnd-0.001 {
			break
		}
		d.pdf.Line(x, y0, x, y0+h)
	}

	if outerBottom {
		d.setGridOuter()
	} else {
		d.setGridInner()
	}
	d.pdf.Line(marginL, y0+h, xEnd, y0+h)
	d.resetGrid()
}

// drawTotalsRow renders the shared pale-yellow totals row: the first five
// columns (item/HSN/qty/rate/discount) are merged into a single
// right-aligned "Total" label cell, and the remaining columns show the
// invoice-level sums.
func (d *doc) drawTotalsRow(cols []tableColumn, result calc.InvoiceResult) {
	x0 := marginL
	y0 := d.pdf.GetY()
	h := tableLineH * 1.4

	d.pdf.SetFillColor(colorHeaderFill[0], colorHeaderFill[1], colorHeaderFill[2])
	d.pdf.Rect(x0, y0, tableWidth(cols), h, "F")

	d.pdf.SetFont(FontFamily, "B", 8)
	d.setNormalText()

	mergeCount := 5 // item, hsn, qty, rate, discount
	if len(cols) < mergeCount {
		mergeCount = len(cols)
	}
	mergedW := 0.0
	for i := 0; i < mergeCount; i++ {
		mergedW += cols[i].Width
	}
	d.pdf.SetXY(x0, y0)
	d.pdf.CellFormat(mergedW, h, LabelTableTotal, "", 0, "R", false, 0, "")

	x := x0 + mergedW
	for i := mergeCount; i < len(cols); i++ {
		c := cols[i]
		text := totalsValueFor(c.ID, result)
		d.pdf.SetXY(x, y0)
		d.pdf.CellFormat(c.Width, h, text, "", 0, c.Align, false, 0, "")
		x += c.Width
	}

	d.drawRowGrid(cols, y0, h, true)
	d.pdf.SetXY(x0, y0+h)
}

func totalsValueFor(id string, result calc.InvoiceResult) string {
	switch id {
	case "taxable":
		return money.FormatINR(result.TaxableINR)
	case "igst":
		return money.FormatINR(result.IGST)
	case "cgst":
		return money.FormatINR(result.CGST)
	case "sgst":
		return money.FormatINR(result.SGST)
	case "cess":
		return money.FormatINR(result.CESS)
	case "total_inr":
		return money.FormatINR(result.TotalINR)
	case "total_usd":
		return money.FormatUSD(result.TotalUSD)
	}
	return ""
}

// hasGroups reports whether any column in cols belongs to a merged group
// header (currently only the export "Total" group).
func hasGroups(cols []tableColumn) bool {
	for _, c := range cols {
		if c.Group != "" {
			return true
		}
	}
	return false
}

func tableWidth(cols []tableColumn) float64 {
	w := 0.0
	for _, c := range cols {
		w += c.Width
	}
	return w
}

// drawTableHeader draws the two-tier column header (a merged group row only
// where needed, e.g. "Total" spanning INR/USD) with the pale-yellow fill,
// and returns after leaving the cursor at the top of the first body row.
func (d *doc) drawTableHeader(cols []tableColumn) {
	groups := hasGroups(cols)
	h1 := 0.0
	if groups {
		h1 = headerGroupH
	}
	h2 := headerRowH
	totalH := h1 + h2

	x0 := marginL
	y0 := d.pdf.GetY()

	d.pdf.SetFillColor(colorHeaderFill[0], colorHeaderFill[1], colorHeaderFill[2])
	d.pdf.Rect(x0, y0, tableWidth(cols), totalH, "F")

	d.pdf.SetFont(FontFamily, "B", 7.5)
	d.setNormalText()

	x := x0
	i := 0
	for i < len(cols) {
		c := cols[i]
		if groups && c.Group != "" {
			// Merge this column with any immediately following columns
			// sharing the same group label.
			j := i + 1
			gw := c.Width
			for j < len(cols) && cols[j].Group == c.Group {
				gw += cols[j].Width
				j++
			}
			d.pdf.SetXY(x, y0)
			d.pdf.CellFormat(gw, h1, c.Group, "", 0, "C", false, 0, "")
			d.setGridInner()
			d.pdf.Line(x, y0+h1, x+gw, y0+h1)
			// Sub-headers for the grouped columns.
			sx := x
			for k := i; k < j; k++ {
				d.pdf.SetXY(sx, y0+h1)
				d.drawHeaderCellText(cols[k].Header, sx, y0+h1, cols[k].Width, h2)
				sx += cols[k].Width
			}
			x += gw
			i = j
			continue
		}
		// Ungrouped column: header text spans the full header height.
		d.drawHeaderCellText(c.Header, x, y0, c.Width, totalH)
		x += c.Width
		i++
	}

	// Vertical separators: the left/right table edges are drawn heavier
	// (outer frame); every internal column divider is drawn light and thin.
	// A divider that falls *inside* a merged group header (e.g. between the
	// Total group's INR/USD sub-columns) is only drawn below the group
	// label row, so it never strikes through the merged group text.
	xx := x0
	boundaries := make([]float64, 0, len(cols)+1)
	boundaries = append(boundaries, xx)
	for _, c := range cols {
		xx += c.Width
		boundaries = append(boundaries, xx)
	}
	for b := 0; b < len(boundaries); b++ {
		bx := boundaries[b]
		switch {
		case b == 0 || b == len(boundaries)-1:
			d.setGridOuter()
			d.pdf.Line(bx, y0, bx, y0+totalH)
		case groups && cols[b-1].Group != "" && cols[b].Group == cols[b-1].Group:
			// Internal to a merged group: skip the group-label row so the
			// divider never crosses the spanning header text.
			d.setGridInner()
			d.pdf.Line(bx, y0+h1, bx, y0+totalH)
		default:
			d.setGridInner()
			d.pdf.Line(bx, y0, bx, y0+totalH)
		}
	}

	// Top edge is the true outer top of the table; the line under the
	// header (separating it from the first body row) is an internal rule.
	d.setGridOuter()
	d.pdf.Line(x0, y0, xx, y0)
	d.setGridInner()
	d.pdf.Line(x0, y0+totalH, xx, y0+totalH)
	d.resetGrid()

	d.pdf.SetXY(x0, y0+totalH)
}

// drawHeaderCellText vertically centers (possibly two-line, "\n"-separated)
// header text within a cell.
func (d *doc) drawHeaderCellText(text string, x, y, w, h float64) {
	lines := splitLinesRaw(text)
	lineH := 3.5
	totalTextH := float64(len(lines)) * lineH
	startY := y + (h-totalTextH)/2
	for _, ln := range lines {
		d.pdf.SetXY(x, startY)
		d.pdf.CellFormat(w, lineH, ln, "", 0, "C", false, 0, "")
		startY += lineH
	}
}

// cellLines returns the wrapped display lines for one column of one line
// item. fx is the invoice's conversion factor, used to derive the per-unit
// rate and discount INR figures shown in the table (tax itself is always
// computed on lr.TaxableINR, already converted).
func (d *doc) cellLines(col tableColumn, idx int, lr calc.LineResult, li model.LineItem, fx money.Rate) []string {
	switch col.ID {
	case "item":
		return d.wrapText(fmt.Sprintf("%d. %s", idx+1, li.Description), col.Width-2)
	case "hsn":
		return d.wrapText(li.HSNSAC, col.Width-2)
	case "qty":
		return []string{li.Quantity.String(), li.Unit}
	case "rate":
		return []string{money.FormatINR(money.ConvertUSDToINR(li.RateUSD, fx))}
	case "discount":
		return []string{money.FormatINR(money.ConvertUSDToINR(li.DiscountUSD, fx))}
	case "taxable":
		return []string{money.FormatINR(lr.TaxableINR)}
	case "igst":
		return []string{money.FormatINR(lr.IGST), calc.RateLabel(li.TaxRatePct)}
	case "cgst":
		return []string{money.FormatINR(lr.CGST), calc.RateLabel(li.TaxRatePct / 2)}
	case "sgst":
		return []string{money.FormatINR(lr.SGST), calc.RateLabel(li.TaxRatePct / 2)}
	case "cess":
		return []string{money.FormatINR(lr.CESS)}
	case "total_inr":
		return []string{money.FormatINR(lr.TotalINR)}
	case "total_usd":
		return []string{money.FormatUSD(lr.TotalUSD)}
	}
	return []string{""}
}
