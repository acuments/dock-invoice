package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Widget-level helpers that sit on top of the design system in design.go.
// Anything here composes tokens from that file; none of it invents spacing,
// colour, or type sizes of its own.

// formStack stacks form sections on the standard vertical rhythm.
func formStack(items ...fyne.CanvasObject) fyne.CanvasObject {
	return stack(spaceLg, items...)
}

// pathPickerRow is Choose… + path label, used for logo/signature/output paths.
func pathPickerRow(btn *widget.Button, pathLabel *widget.Label) fyne.CanvasObject {
	pathLabel.Importance = widget.LowImportance
	pathLabel.Wrapping = fyne.TextTruncate
	return container.NewBorder(nil, nil, btn, nil, pathLabel)
}

// actionBar pins actions to the bottom of a screen on a solid bar with a top
// hairline, so it reads as pinned chrome rather than a floating button row.
func actionBar(leading, trailing fyne.CanvasObject) fyne.CanvasObject {
	var content fyne.CanvasObject
	switch {
	case leading != nil && trailing != nil:
		content = container.NewBorder(nil, nil, leading, trailing, nil)
	case trailing != nil:
		content = container.NewBorder(nil, nil, nil, trailing, nil)
	default:
		content = leading
	}
	bg := canvas.NewRectangle(themeColor(theme.ColorNameInputBackground))
	return container.NewStack(bg, container.NewVBox(
		rule(),
		padXY(spaceMd, spaceXl, content),
	))
}

// statusBar is the quiet single-line footer used for transient screen status.
func statusBar(status *widget.Label) fyne.CanvasObject {
	status.Importance = widget.LowImportance
	return padXY(spaceXs, spaceXl, status)
}

// showSizedConfirm shows a Save/Cancel dialog resized so multi-line entries
// and form fields actually expand.
func showSizedConfirm(win fyne.Window, title string, content fyne.CanvasObject, size fyne.Size, callback func(bool)) {
	d := dialog.NewCustomConfirm(title, "Save", "Cancel", content, callback, win)
	d.Resize(size)
	d.Show()
}

// multiLineEntry returns a multi-line entry with enough visible rows to type
// into, and word wrap so long addresses stay readable.
func multiLineEntry(text string, rows int) *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.SetText(text)
	e.SetMinRowsVisible(rows)
	e.Wrapping = fyne.TextWrapWord
	return e
}
