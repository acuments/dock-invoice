package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The line-item grid is the one place in the app that really is a table, so it
// is drawn as one: column captions printed once in a header band, then bare
// rows separated by hairlines.
//
// Each row used to be a bordered card that repeated all nine column captions
// ("Description", "HSN", "Qty", …) above its own inputs. On a three-line
// invoice that meant twenty-seven caption strings competing with nine actual
// figures, and every row stood a caption's height taller than it needed to.

// lineItemWeights is the shared column geometry: the header band and every
// data row lay out against this same slice, which is what keeps the captions
// sitting over their columns.
// Description carries the only free prose on the row, so it is given the
// lion's share; the numeric columns need only enough width for their figures.
// Rate is wider than it looks like it needs: domestic invoices are priced in
// rupees, so a rate is routinely six figures ("100000.00") where an export
// invoice's USD rate is four.
var lineItemWeights = []float32{2.85, 0.85, 0.55, 0.5, 1.0, 0.55, 0.42, 0.42, 1.25}

var lineItemCaptions = []string{"Description", "HSN/SAC", "Qty", "Unit", "Rate", "Disc", "Tax%", "Cess%", "Amount"}

// lineItemActionWidth reserves the trailing gutter for each row's delete
// button, and the matching blank space in the header, so the Amount column
// stays aligned with its caption.
const lineItemActionWidth float32 = 34

// lineItemsHeader is the printed column band above the rows.
func lineItemsHeader() fyne.CanvasObject {
	cells := make([]fyne.CanvasObject, len(lineItemCaptions))
	for i, c := range lineItemCaptions {
		t := kickerText(c)
		// The figures column is right-aligned, so its caption is too.
		if i == len(lineItemCaptions)-1 {
			t.Alignment = fyne.TextAlignTrailing
		}
		cells[i] = t
	}
	band := container.New(&ratioHBox{weights: lineItemWeights, gap: spaceSm}, cells...)
	return stack(spaceXs,
		container.NewBorder(nil, nil, nil, hgap(lineItemActionWidth), band),
		rule(),
	)
}

// buildLineItemCard returns one line row plus the amount label recalc() updates.
func (e *Editor) buildLineItemCard(li *LineItemState) (fyne.CanvasObject, *widget.Label) {
	desc := widget.NewEntry()
	desc.SetPlaceHolder("Description of goods or services")
	desc.SetText(li.Description)
	desc.OnChanged = func(v string) { li.Description = v }

	hsn := widget.NewEntry()
	hsn.SetPlaceHolder("998314")
	hsn.SetText(li.HSNSAC)
	hsn.OnChanged = func(v string) { li.HSNSAC = v }

	qty := newDecimalEntry("1.00")
	qty.SetText(li.Quantity)
	qty.OnChanged = func(v string) { li.Quantity = v; e.recalc() }

	unit := widget.NewEntry()
	unit.SetPlaceHolder("UNT")
	unit.SetText(li.Unit)
	unit.OnChanged = func(v string) { li.Unit = v }

	rate := newDecimalEntry("0.00")
	rate.SetPlaceHolder("0.00")
	rate.SetText(li.RateUSD)
	rate.OnChanged = func(v string) { li.RateUSD = v; e.recalc() }

	discount := newDecimalEntry("0.00")
	discount.SetPlaceHolder("0.00")
	discount.SetText(li.DiscountUSD)
	discount.OnChanged = func(v string) { li.DiscountUSD = v; e.recalc() }

	tax := newDecimalEntry("18")
	tax.SetPlaceHolder("18")
	tax.SetText(li.TaxRatePct)
	tax.OnChanged = func(v string) { li.TaxRatePct = v; e.recalc() }

	cess := newDecimalEntry("0")
	cess.SetPlaceHolder("0")
	cess.SetText(li.CessRatePct)
	cess.OnChanged = func(v string) { li.CessRatePct = v; e.recalc() }

	// amount stays a *widget.Label (not canvas text) so tests can read its
	// .Text after recalc. It is set in ink and bold rather than the accent
	// colour: the accent means "actionable", and a computed figure is not.
	amount := widget.NewLabel("—")
	amount.Alignment = fyne.TextAlignTrailing
	amount.TextStyle = fyne.TextStyle{Bold: true}

	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		e.state.RemoveLineState(li)
		e.refreshItems()
		e.recalc()
	})
	del.Importance = widget.LowImportance

	fields := container.New(&ratioHBox{weights: lineItemWeights, gap: spaceSm},
		desc, hsn, qty, unit, rate, discount, tax, cess,
		container.NewCenter(amount),
	)
	body := container.NewBorder(nil, nil, nil, fixedWidth(lineItemActionWidth, del), fields)
	return body, amount
}
