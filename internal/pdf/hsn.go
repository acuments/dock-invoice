package pdf

import (
	"dock-invoice/internal/calc"
	"dock-invoice/internal/money"
)

// HSN summary column widths total contentW (190mm) for both tax layouts.
const (
	hsnColCode     = 28.0
	hsnColTaxable  = 32.0
	hsnColRate     = 16.0
	hsnColAmount   = 28.0
	hsnColTotalTax = 42.0 // CGST/SGST layout: 28+32+16+28+16+28+42 = 190

	hsnIGSTColTaxable  = 42.0
	hsnIGSTColRate     = 24.0
	hsnIGSTColAmount   = 42.0
	hsnIGSTColTotalTax = 54.0 // 28+42+24+42+54 = 190

	hsnHeaderH = 5.0
	hsnGroupH  = 4.5
	hsnRowH    = 5.5
	hsnTitleH  = 5.5
	hsnGap     = 3.0
)

// hsnCol describes one column of the HSN/SAC summary table.
type hsnCol struct {
	ID     string
	Header string
	Group  string // non-empty: merged with siblings under a spanning header
	Width  float64
	Align  string
}

// drawHSNSummary renders the HSN/SAC-wise tax breakdown for domestic
// invoices when Settings.IncludeHSNSACSummary is on. Layout follows the
// statutory table: code, taxable amount, per-tax rate+amount columns,
// and total tax; CGST/SGST for intra-state, IGST for inter-state.
func (d *doc) drawHSNSummary(result calc.InvoiceResult) {
	rows := calc.HSNSummary(result)
	if len(rows) == 0 {
		return
	}
	cols := buildHSNColumns(result.Mode)

	// Title sits above the grid; keep title + header + at least one data
	// row together so the section never starts with a lone title.
	need := hsnGap + hsnTitleH + hsnTableHeaderHeight(cols) + hsnRowH
	d.ensureSpace(need)

	y := d.pdf.GetY() + hsnGap
	d.pdf.SetFont(FontFamily, "B", 9)
	d.setNormalText()
	d.pdf.SetXY(marginL, y)
	d.pdf.CellFormat(contentW, hsnTitleH, LabelHSNSummary, "", 0, "L", false, 0, "")
	d.pdf.SetXY(marginL, y+hsnTitleH)

	d.drawHSNHeader(cols)

	for _, row := range rows {
		if d.ensureSpace(hsnRowH) {
			d.drawHSNHeader(cols)
		}
		d.drawHSNDataRow(cols, row)
	}

	if d.ensureSpace(hsnRowH) {
		d.drawHSNHeader(cols)
	}
	d.drawHSNTotalRow(cols, rows)
	d.pdf.SetXY(marginL, d.pdf.GetY()+2)
}

func buildHSNColumns(mode calc.TaxMode) []hsnCol {
	if mode == calc.TaxCGSTSGST {
		return []hsnCol{
			{ID: "hsn", Header: ColHSNSummaryCode, Width: hsnColCode, Align: "L"},
			{ID: "taxable", Header: ColHSNTaxableAmount, Width: hsnColTaxable, Align: "R"},
			{ID: "cgst_rate", Header: ColHSNRate, Group: ColCGSTGroup, Width: hsnColRate, Align: "C"},
			{ID: "cgst_amt", Header: ColHSNAmount, Group: ColCGSTGroup, Width: hsnColAmount, Align: "R"},
			{ID: "sgst_rate", Header: ColHSNRate, Group: ColSGSTGroup, Width: hsnColRate, Align: "C"},
			{ID: "sgst_amt", Header: ColHSNAmount, Group: ColSGSTGroup, Width: hsnColAmount, Align: "R"},
			{ID: "total_tax", Header: ColHSNTotalTax, Width: hsnColTotalTax, Align: "R"},
		}
	}
	return []hsnCol{
		{ID: "hsn", Header: ColHSNSummaryCode, Width: hsnColCode, Align: "L"},
		{ID: "taxable", Header: ColHSNTaxableAmount, Width: hsnIGSTColTaxable, Align: "R"},
		{ID: "igst_rate", Header: ColHSNRate, Group: ColIGSTGroup, Width: hsnIGSTColRate, Align: "C"},
		{ID: "igst_amt", Header: ColHSNAmount, Group: ColIGSTGroup, Width: hsnIGSTColAmount, Align: "R"},
		{ID: "total_tax", Header: ColHSNTotalTax, Width: hsnIGSTColTotalTax, Align: "R"},
	}
}

func hsnTableWidth(cols []hsnCol) float64 {
	w := 0.0
	for _, c := range cols {
		w += c.Width
	}
	return w
}

func hsnHasGroups(cols []hsnCol) bool {
	for _, c := range cols {
		if c.Group != "" {
			return true
		}
	}
	return false
}

func hsnTableHeaderHeight(cols []hsnCol) float64 {
	if hsnHasGroups(cols) {
		return hsnGroupH + hsnHeaderH
	}
	return hsnHeaderH
}

func (d *doc) drawHSNHeader(cols []hsnCol) {
	groups := hsnHasGroups(cols)
	h1 := 0.0
	if groups {
		h1 = hsnGroupH
	}
	h2 := hsnHeaderH
	totalH := h1 + h2

	x0 := marginL
	y0 := d.pdf.GetY()

	d.pdf.SetFillColor(colorHeaderFill[0], colorHeaderFill[1], colorHeaderFill[2])
	d.pdf.Rect(x0, y0, hsnTableWidth(cols), totalH, "F")

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
			d.setGridInner()
			d.pdf.Line(x, y0+h1, x+gw, y0+h1)
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
		d.drawHeaderCellText(c.Header, x, y0, c.Width, totalH)
		x += c.Width
		i++
	}

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
			d.setGridInner()
			d.pdf.Line(bx, y0+h1, bx, y0+totalH)
		default:
			d.setGridInner()
			d.pdf.Line(bx, y0, bx, y0+totalH)
		}
	}

	d.setGridOuter()
	d.pdf.Line(x0, y0, xx, y0)
	d.setGridInner()
	d.pdf.Line(x0, y0+totalH, xx, y0+totalH)
	d.resetGrid()

	d.pdf.SetXY(x0, y0+totalH)
}

func (d *doc) drawHSNDataRow(cols []hsnCol, row calc.HSNSummaryRow) {
	y0 := d.pdf.GetY()
	d.pdf.SetFont(FontFamily, "", 8)
	d.setNormalText()
	d.drawHSNCells(cols, y0, hsnRowH, func(id string) string {
		return hsnCellValue(id, row)
	})
	d.drawHSNRowGrid(cols, y0, hsnRowH, false)
	d.pdf.SetXY(marginL, y0+hsnRowH)
}

// hsnTotals holds the summed figure for the Total row.
type hsnTotals struct {
	TaxableINR money.Amount
	CGST       money.Amount
	SGST       money.Amount
	IGST       money.Amount
	TotalTax   money.Amount
}

func sumHSNRows(rows []calc.HSNSummaryRow) hsnTotals {
	var t hsnTotals
	for _, r := range rows {
		t.TaxableINR += r.TaxableINR
		t.CGST += r.CGST
		t.SGST += r.SGST
		t.IGST += r.IGST
		t.TotalTax += r.TotalTax
	}
	return t
}

// drawHSNTotalRow renders the bold totals row at the foot of the HSN table.
func (d *doc) drawHSNTotalRow(cols []hsnCol, rows []calc.HSNSummaryRow) {
	y0 := d.pdf.GetY()
	totals := sumHSNRows(rows)

	d.pdf.SetFillColor(colorHeaderFill[0], colorHeaderFill[1], colorHeaderFill[2])
	d.pdf.Rect(marginL, y0, hsnTableWidth(cols), hsnRowH, "F")

	d.pdf.SetFont(FontFamily, "B", 8)
	d.setNormalText()
	d.drawHSNCells(cols, y0, hsnRowH, func(id string) string {
		switch id {
		case "hsn":
			return LabelTableTotal
		case "taxable":
			return money.FormatINR(totals.TaxableINR)
		case "cgst_amt":
			return money.FormatINR(totals.CGST)
		case "sgst_amt":
			return money.FormatINR(totals.SGST)
		case "igst_amt":
			return money.FormatINR(totals.IGST)
		case "total_tax":
			return money.FormatINR(totals.TotalTax)
		case "cgst_rate", "sgst_rate", "igst_rate":
			return ""
		}
		return ""
	})
	d.drawHSNRowGrid(cols, y0, hsnRowH, true)
	d.pdf.SetXY(marginL, y0+hsnRowH)
}

func (d *doc) drawHSNCells(cols []hsnCol, y0, h float64, value func(id string) string) {
	x := marginL
	for _, c := range cols {
		d.pdf.SetXY(x, y0)
		d.pdf.CellFormat(c.Width, h, value(c.ID), "", 0, c.Align, false, 0, "")
		x += c.Width
	}
}

func (d *doc) drawHSNRowGrid(cols []hsnCol, y0, h float64, outerBottom bool) {
	x := marginL
	xEnd := marginL + hsnTableWidth(cols)

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

func hsnCellValue(id string, row calc.HSNSummaryRow) string {
	half := row.TaxRatePct / 2
	switch id {
	case "hsn":
		return row.HSNSAC
	case "taxable":
		return money.FormatINR(row.TaxableINR)
	case "cgst_rate":
		return calc.PercentLabel(half)
	case "cgst_amt":
		return money.FormatINR(row.CGST)
	case "sgst_rate":
		return calc.PercentLabel(half)
	case "sgst_amt":
		return money.FormatINR(row.SGST)
	case "igst_rate":
		return calc.PercentLabel(row.TaxRatePct)
	case "igst_amt":
		return money.FormatINR(row.IGST)
	case "total_tax":
		return money.FormatINR(row.TotalTax)
	}
	return ""
}

// hsnSummaryHeight estimates vertical space for the HSN section so the
// footer keep-together pass accounts for it.
func hsnSummaryHeight(result calc.InvoiceResult) float64 {
	rows := calc.HSNSummary(result)
	if len(rows) == 0 {
		return 0
	}
	cols := buildHSNColumns(result.Mode)
	// title + header + n data rows + total row + trailing gap
	return hsnGap + hsnTitleH + hsnTableHeaderHeight(cols) +
		float64(len(rows)+1)*hsnRowH + 2
}
