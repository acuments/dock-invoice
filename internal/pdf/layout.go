// Package pdf renders an invoice to PDF bytes, reproducing the layout of
// Reference layout (INV-01.pdf): A4 portrait, 10mm margins, Noto Sans throughout so ₹
// (U+20B9) always renders regardless of platform system fonts.
package pdf

import (
	"github.com/go-pdf/fpdf"
)

const (
	marginL      = 10.0
	marginR      = 10.0
	marginTop    = 10.0
	marginBottom = 12.0

	pageW = 210.0
	pageH = 297.0

	contentW = pageW - marginL - marginR
)

// Colors approximate the sample document's warm red/orange accent and pale
// yellow table header fill.
var (
	colorAccent     = [3]int{176, 44, 34} // titles, dividers, company name, bank box border
	colorText       = [3]int{20, 20, 20}
	colorMuted      = [3]int{90, 90, 90}
	colorHeaderFill = [3]int{252, 240, 200} // pale yellow table header / totals row
	colorBorder     = [3]int{170, 170, 170}

	// Table grid styling: a crisper outer frame with much lighter internal
	// rules, matching the reference document (which uses light internal
	// grid lines and reserves visual weight for the table's outer border).
	colorTableOuter = [3]int{120, 120, 120}
	colorTableInner = [3]int{218, 218, 218}
)

const (
	lineWidthTableOuter = 0.3
	lineWidthTableInner = 0.12
)

// setGridOuter selects the draw style used for the table's outer frame
// (top/bottom/left/right edges).
func (d *doc) setGridOuter() {
	d.pdf.SetDrawColor(colorTableOuter[0], colorTableOuter[1], colorTableOuter[2])
	d.pdf.SetLineWidth(lineWidthTableOuter)
}

// setGridInner selects the lighter, thinner draw style used for internal
// table rules (column dividers, row separators, header sub-divisions).
func (d *doc) setGridInner() {
	d.pdf.SetDrawColor(colorTableInner[0], colorTableInner[1], colorTableInner[2])
	d.pdf.SetLineWidth(lineWidthTableInner)
}

// resetGrid restores the document's default draw style, used outside the
// items table.
func (d *doc) resetGrid() {
	d.pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	d.pdf.SetLineWidth(0.2)
}

// doc wraps an *fpdf.Fpdf with the layout state shared across band-drawing
// functions.
type doc struct {
	pdf *fpdf.Fpdf
}

func newDoc() *doc {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetAutoPageBreak(false, marginBottom)
	p.SetMargins(marginL, marginTop, marginR)
	registerFonts(p)
	p.SetFont(FontFamily, "", 9)
	p.SetTextColor(colorText[0], colorText[1], colorText[2])
	p.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	p.SetLineWidth(0.2)
	return &doc{pdf: p}
}

func (d *doc) setAccentText() {
	d.pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
}

func (d *doc) setNormalText() {
	d.pdf.SetTextColor(colorText[0], colorText[1], colorText[2])
}

func (d *doc) setMutedText() {
	d.pdf.SetTextColor(colorMuted[0], colorMuted[1], colorMuted[2])
}

// spaceEpsilon absorbs floating-point noise in ensureSpace: content sized to
// exactly fill the remaining space (the classic layout pads its items table
// to do precisely that) accumulates rounding error well below print
// resolution, and must not be bumped to a fresh page over it.
const spaceEpsilon = 0.01

// ensureSpace adds a new page if fewer than `needed` mm remain above the
// bottom margin, returning true if a page break happened.
func (d *doc) ensureSpace(needed float64) bool {
	_, pageH := d.pdf.GetPageSize()
	_, _, _, bm := d.pdf.GetMargins()
	if d.pdf.GetY()+needed > pageH-bm+spaceEpsilon {
		d.pdf.AddPage()
		return true
	}
	return false
}

// wrapText splits text into lines that fit within width w using the
// currently selected font, honoring explicit newlines first.
func (d *doc) wrapText(text string, w float64) []string {
	if text == "" {
		return []string{""}
	}
	var out []string
	for _, raw := range splitLinesRaw(text) {
		if raw == "" {
			out = append(out, "")
			continue
		}
		out = append(out, d.pdf.SplitText(raw, w)...)
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

func splitLinesRaw(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

// hline draws a horizontal divider line across the content width at the
// current y position (accent colored, matching the sample's red rules).
func (d *doc) hline(y float64) {
	d.pdf.SetDrawColor(colorAccent[0], colorAccent[1], colorAccent[2])
	d.pdf.SetLineWidth(0.3)
	d.pdf.Line(marginL, y, pageW-marginR, y)
	d.pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	d.pdf.SetLineWidth(0.2)
}

// labelValue draws a bold label followed by a normal-weight value on the
// same baseline, returning the x position just after the printed text.
func (d *doc) labelValue(x, y float64, label, value string, labelSize, valueSize float64) float64 {
	d.pdf.SetXY(x, y)
	d.pdf.SetFont(FontFamily, "B", labelSize)
	d.setMutedText()
	d.pdf.CellFormat(d.pdf.GetStringWidth(label)+1.5, valueSize/2.2, label, "", 0, "L", false, 0, "")
	d.pdf.SetFont(FontFamily, "", valueSize)
	d.setNormalText()
	vw := d.pdf.GetStringWidth(value) + 1
	d.pdf.CellFormat(vw, valueSize/2.2, value, "", 0, "L", false, 0, "")
	return d.pdf.GetX()
}
