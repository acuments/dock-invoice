package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// tappableArea makes an arbitrary canvas object clickable without dressing it
// as a button. The invoice list's column headers need this: they are sort
// controls, but drawing them as five buttons would put more chrome above the
// data than the data itself carries.
type tappableArea struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTappableArea(content fyne.CanvasObject, onTap func()) *tappableArea {
	t := &tappableArea{content: content, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

func (t *tappableArea) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

// Cursor is the only affordance an undressed control gets: without it a
// column header looks exactly like the printed caption it is styled as, and
// nothing but the page subtitle says the sort is there at all.
func (t *tappableArea) Cursor() desktop.Cursor { return desktop.PointerCursor }
