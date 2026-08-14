package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

var (
	_ fyne.Widget         = (*invoiceRow)(nil)
	_ fyne.DoubleTappable = (*invoiceRow)(nil)
	_ desktop.Mouseable   = (*invoiceRow)(nil)
	_ desktop.Cursorable  = (*invoiceRow)(nil)
)

// invoiceRow is one row of the invoices list: the nine texts on the shared
// column weights, a bottom hairline, and an invisible sizer that sets the row
// height. Row backgrounds — hover and selection — are left to widget.List,
// which paints them behind this whole object.
//
// It is a widget rather than a bare container for two reasons:
//
//   - It owns its cells, so the list's update callback addresses a column by
//     index instead of walking an anonymous container tree by position (the
//     previous chain of Objects[n] assertions would have panicked on any
//     change to the row's nesting).
//   - It handles its own pointer events, which is what makes double-click to
//     edit possible: widget.List offers no double-tap hook. Fyne delivers a
//     click to exactly one object — the innermost that handles pointer events
//     — so a child declaring only DoubleTappable would swallow the tap the
//     list row needs in order to select itself. Selection therefore happens
//     here, on mouse-down: Fyne must hold a Tapped back for the whole
//     double-tap interval, which would make every selection land a beat late.
type invoiceRow struct {
	widget.BaseWidget

	cells   []*canvas.Text
	content fyne.CanvasObject

	// id is the list index this recycled row currently shows, or -1 before
	// its first update. The pointer handlers report it back to the screen.
	id         widget.ListItemID
	onSelect   func(widget.ListItemID)
	onActivate func(widget.ListItemID)
}

// newInvoiceRow builds an empty row. onSelect is called on a click and
// onActivate on a double-click, each with the row's current list index.
func newInvoiceRow(onSelect, onActivate func(widget.ListItemID)) *invoiceRow {
	r := &invoiceRow{id: -1, onSelect: onSelect, onActivate: onActivate}

	r.cells = make([]*canvas.Text, len(invoiceListColumns))
	objects := make([]fyne.CanvasObject, len(invoiceListColumns))
	for i := range r.cells {
		r.cells[i] = text("", textBody, false, themeColor(theme.ColorNameForeground))
		objects[i] = r.cells[i]
	}

	band := container.New(&ratioHBox{weights: invoiceListWeights, gap: spaceMd}, objects...)
	sizer := canvas.NewRectangle(color.Transparent)
	sizer.SetMinSize(fyne.NewSize(1, invoiceRowHeight))
	r.content = container.NewStack(sizer, container.NewBorder(nil, rule(), nil, nil,
		vcenter(padXY(0, invoiceListGutter, band)),
	))

	r.ExtendBaseWidget(r)
	return r
}

func (r *invoiceRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

// set fills the row for inv, which the list shows at index id. Every visual
// property a cell can carry is written on every update, never only in the
// branch that wants it: widget.List recycles rows, so a bold weight or a type
// tint left over from another invoice would otherwise leak into this one.
func (r *invoiceRow) set(id widget.ListItemID, inv model.Invoice) {
	r.id = id
	result := calc.ComputeInvoice(&inv)

	usd := "—"
	if inv.Currency == "USD" {
		usd = "$ " + money.FormatUSD(result.TotalUSD)
	}
	// CGST/SGST/IGST are never all nonzero on the same invoice (see
	// calc.ComputeLine: intra-state domestic charges CGST+SGST, everything
	// else charges IGST alone or nothing under LUT) but the table always
	// shows all three columns, so the two that don't apply to this invoice
	// need their own "not applicable" rendering — same em-dash convention as
	// the USD column above.
	cgst, sgst, igst := "—", "—", "—"
	if result.CGST != 0 {
		cgst = "₹ " + money.FormatINR(result.CGST)
	}
	if result.SGST != 0 {
		sgst = "₹ " + money.FormatINR(result.SGST)
	}
	if result.IGST != 0 {
		igst = "₹ " + money.FormatINR(result.IGST)
	}
	_, typeFG := invoiceTypeTone(inv.Type).colors()

	for col, txt := range r.cells {
		txt.Alignment = invoiceListColumnAlign[col]
		txt.TextStyle = fyne.TextStyle{}
		txt.TextSize = textBody
		txt.Color = themeColor(theme.ColorNameForeground)

		switch col {
		case colDate:
			txt.Color = themeColor(theme.ColorNamePlaceHolder)
			txt.Text = inv.Date.Format("02 Jan 2006")
		case colNumber:
			txt.TextStyle = fyne.TextStyle{Bold: true}
			txt.Text = inv.Number
		case colCustomer:
			txt.Text = inv.Customer.Name
		case colType:
			// Tinted rather than badged: a pill on every row would out-weigh
			// the figures, but the tone still lets you pick the domestic
			// invoices out of a long export series.
			txt.Color = typeFG
			txt.TextSize = textCaption
			txt.TextStyle = fyne.TextStyle{Bold: true}
			txt.Text = InvoiceTypeLabel(inv.Type)
		case colCGST:
			txt.Text = cgst
			if result.CGST == 0 {
				txt.Color = themeColor(theme.ColorNamePlaceHolder)
			}
		case colSGST:
			txt.Text = sgst
			if result.SGST == 0 {
				txt.Color = themeColor(theme.ColorNamePlaceHolder)
			}
		case colIGST:
			txt.Text = igst
			if result.IGST == 0 {
				txt.Color = themeColor(theme.ColorNamePlaceHolder)
			}
		case colINRTotal:
			txt.Text = "₹ " + money.FormatINR(result.GrandTotal)
		case colUSDTotal:
			txt.Color = themeColor(theme.ColorNamePlaceHolder)
			txt.Text = usd
		}
		txt.Refresh()
	}
}

// MouseDown selects the row. Secondary clicks are ignored: a right-click that
// silently moved the selection would arm the toolbar's Delete against a
// different invoice than the one the user thinks is chosen.
func (r *invoiceRow) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button != desktop.MouseButtonPrimary {
		return
	}
	if r.onSelect != nil && r.id >= 0 {
		r.onSelect(r.id)
	}
}

func (r *invoiceRow) MouseUp(*desktop.MouseEvent) {}

// DoubleTapped opens the row's invoice in the editor — the shortcut for the
// toolbar's select-then-Edit, which is the list's most common errand.
func (r *invoiceRow) DoubleTapped(*fyne.PointEvent) {
	if r.onActivate != nil && r.id >= 0 {
		r.onActivate(r.id)
	}
}

// Cursor shows the pointer over a row, so the rows read as things you can act
// on rather than as printed lines.
func (r *invoiceRow) Cursor() desktop.Cursor { return desktop.PointerCursor }
