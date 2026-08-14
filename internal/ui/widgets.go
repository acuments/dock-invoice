package ui

import (
	"fyne.io/fyne/v2/widget"
)

// newDecimalEntry returns a plain decimal Entry for quantity/rate/discount/
// percent/factor fields. Live Fyne Validators are intentionally omitted so
// ActionItem check/error icons never appear; parse/validation happens on save
// via EditorState.Validate / Build (and callers may use money.Parse*). Empty
// is allowed (EditorState treats blank as zero); OnChanged is wired by the
// caller to trigger recalc.
func newDecimalEntry(placeholder string) *widget.Entry {
	e := widget.NewEntry()
	e.SetPlaceHolder(placeholder)
	return e
}

// newRequiredEntry returns a plain widget.Entry. Live Fyne Validators are
// intentionally omitted so ActionItem icons never appear; requiredness is
// enforced on save via EditorState.Validate / Build.
func newRequiredEntry(placeholder string) *widget.Entry {
	e := widget.NewEntry()
	e.SetPlaceHolder(placeholder)
	return e
}
