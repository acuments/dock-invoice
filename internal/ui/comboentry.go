package ui

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// comboOption is one entry in a searchComboBox's option list. Label is the
// primary display/search text; Detail is optional secondary text (e.g. an
// HSN/SAC or SKU code plus a formatted rate) shown alongside the label in
// the dropdown and also matched against while filtering.
type comboOption struct {
	Label  string
	Detail string
}

// comboMenuLimit caps how many rows a single dropdown render shows.
const comboMenuLimit = 40

// comboListMaxRows limits the visible height of the suggestion list.
const comboListMaxRows = 6

// comboListRowHeight is the per-row height used to size the suggestion list.
const comboListRowHeight float32 = 32

// matchComboOptions returns the indices into options whose Label or Detail
// contains query (case-insensitively), in original order, capped at limit.
func matchComboOptions(options []comboOption, query string, limit int) []int {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []int
	for i, opt := range options {
		if q != "" &&
			!strings.Contains(strings.ToLower(opt.Label), q) &&
			!strings.Contains(strings.ToLower(opt.Detail), q) {
			continue
		}
		out = append(out, i)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// searchComboBox is a type-to-filter combobox. While the suggestion list is
// open it takes part in layout (MinSize grows) so later form rows cannot
// paint through a floating overlay. Canvas PopUps were rejected because they
// steal focus and break continuous typing.
type searchComboBox struct {
	widget.BaseWidget

	entry     *focusAwareEntry
	list      *widget.List
	listPanel fyne.CanvasObject
	listBG    *canvas.Rectangle

	options  []comboOption
	filtered []int
	status   string

	onPick      func(index int)
	onTyped     func(query string)
	empty       string
	clearOnPick bool

	listOpen     bool
	ignoreChange bool
	closeTimer   *time.Timer
}

type focusAwareEntry struct {
	widget.Entry
	onFocusLost func()
}

func newFocusAwareEntry() *focusAwareEntry {
	e := &focusAwareEntry{}
	e.ExtendBaseWidget(e)
	return e
}

func (e *focusAwareEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onFocusLost != nil {
		e.onFocusLost()
	}
}

func (e *focusAwareEntry) CreateRenderer() fyne.WidgetRenderer {
	return e.Entry.CreateRenderer()
}

func newSearchComboBox(placeholder, emptyMessage string, options []comboOption, onPick func(index int)) *searchComboBox {
	c := &searchComboBox{options: options, onPick: onPick, empty: emptyMessage, clearOnPick: true}
	c.ExtendBaseWidget(c)

	c.entry = newFocusAwareEntry()
	c.entry.SetPlaceHolder(placeholder)
	c.entry.OnChanged = c.onQueryChanged
	c.entry.onFocusLost = c.scheduleClose

	btn := widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), func() {
		if c.listOpen {
			c.closeList()
			return
		}
		// Chevron shows the full list; typing still filters via onQueryChanged.
		c.openList("")
	})
	btn.Importance = widget.LowImportance
	c.entry.ActionItem = btn

	c.list = widget.NewList(
		func() int { return c.listLength() },
		func() fyne.CanvasObject {
			l := widget.NewLabel("")
			l.Truncation = fyne.TextTruncateEllipsis
			return l
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			c.updateListRow(id, obj.(*widget.Label))
		},
	)
	c.list.OnSelected = func(id widget.ListItemID) {
		if len(c.filtered) == 0 {
			return
		}
		if int(id) < 0 || int(id) >= len(c.filtered) {
			return
		}
		c.pick(c.filtered[id])
	}

	c.listBG = canvas.NewRectangle(themeColor(theme.ColorNameInputBackground))
	c.listBG.StrokeWidth = 1
	c.listBG.CornerRadius = theme.Size(theme.SizeNameMenuRadius)
	c.syncListChrome()
	c.listPanel = container.NewStack(c.listBG, c.list)
	c.listPanel.Hide()

	return c
}

func (c *searchComboBox) CreateRenderer() fyne.WidgetRenderer {
	return &searchComboBoxRenderer{
		objects: []fyne.CanvasObject{c.entry, c.listPanel},
		combo:   c,
	}
}

type searchComboBoxRenderer struct {
	objects []fyne.CanvasObject
	combo   *searchComboBox
}

func (r *searchComboBoxRenderer) Layout(size fyne.Size) {
	c := r.combo
	entryH := c.entry.MinSize().Height
	c.entry.Move(fyne.NewPos(0, 0))
	c.entry.Resize(fyne.NewSize(size.Width, entryH))

	if c.listOpen {
		listH := size.Height - entryH
		if listH < 1 {
			listH = c.listSize().Height
		}
		c.listPanel.Move(fyne.NewPos(0, entryH))
		c.listPanel.Resize(fyne.NewSize(size.Width, listH))
		c.listPanel.Show()
	} else {
		c.listPanel.Hide()
	}
}

func (r *searchComboBoxRenderer) MinSize() fyne.Size {
	s := r.combo.entry.MinSize()
	if r.combo.listOpen {
		s.Height += r.combo.listSize().Height
	}
	return s
}

func (r *searchComboBoxRenderer) Refresh() {
	r.combo.syncListChrome()
	r.Layout(r.combo.Size())
}

func (r *searchComboBoxRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *searchComboBoxRenderer) Destroy() {}

func (c *searchComboBox) SetOptions(options []comboOption) {
	c.options = options
	if c.listOpen {
		c.openList(c.entry.Text)
	}
}

func (c *searchComboBox) syncListChrome() {
	if c.listBG == nil {
		return
	}
	c.listBG.FillColor = themeColor(theme.ColorNameInputBackground)
	c.listBG.StrokeColor = themeColor(theme.ColorNameInputBorder)
	c.listBG.Refresh()
}

func (c *searchComboBox) refreshLayout() {
	c.Refresh()
	if fyne.CurrentApp() == nil {
		return
	}
	canv := fyne.CurrentApp().Driver().CanvasForObject(c)
	if canv == nil {
		return
	}
	if content := canv.Content(); content != nil {
		content.Refresh()
	}
}

func (c *searchComboBox) setText(s string) {
	c.ignoreChange = true
	c.entry.SetText(s)
	c.ignoreChange = false
}

func (c *searchComboBox) onQueryChanged(query string) {
	if c.ignoreChange {
		return
	}
	if c.onTyped != nil {
		c.onTyped(query)
	}
	if strings.TrimSpace(query) == "" {
		c.closeList()
		return
	}
	c.openList(query)
}

func (c *searchComboBox) openList(query string) {
	c.cancelClose()
	c.applyFilter(query)
	c.list.UnselectAll()
	c.list.Refresh()
	c.listOpen = true
	c.refreshLayout()
}

func (c *searchComboBox) applyFilter(query string) {
	if len(c.options) == 0 {
		c.filtered = nil
		c.status = c.empty
		if c.status == "" {
			c.status = "No options available"
		}
		return
	}
	c.filtered = matchComboOptions(c.options, query, comboMenuLimit)
	if len(c.filtered) == 0 {
		c.status = "No matches"
	} else {
		c.status = ""
	}
}

func (c *searchComboBox) closeList() {
	c.cancelClose()
	if !c.listOpen {
		return
	}
	c.listOpen = false
	c.filtered = nil
	c.status = ""
	c.refreshLayout()
}

func (c *searchComboBox) scheduleClose() {
	c.cancelClose()
	c.closeTimer = time.AfterFunc(150*time.Millisecond, func() {
		fyne.Do(c.closeList)
	})
}

func (c *searchComboBox) cancelClose() {
	if c.closeTimer != nil {
		c.closeTimer.Stop()
		c.closeTimer = nil
	}
}

func (c *searchComboBox) listLength() int {
	if len(c.filtered) > 0 {
		return len(c.filtered)
	}
	if c.status != "" {
		return 1
	}
	return 0
}

func (c *searchComboBox) updateListRow(id widget.ListItemID, label *widget.Label) {
	if len(c.filtered) == 0 {
		label.SetText(c.status)
		return
	}
	opt := c.options[c.filtered[id]]
	text := opt.Label
	if opt.Detail != "" {
		text = opt.Label + "   ·   " + opt.Detail
	}
	label.SetText(text)
}

func (c *searchComboBox) listSize() fyne.Size {
	rows := c.listLength()
	if rows > comboListMaxRows {
		rows = comboListMaxRows
	}
	if rows == 0 {
		rows = 1
	}
	width := c.Size().Width
	if width <= 0 {
		width = c.entry.MinSize().Width
	}
	return fyne.NewSize(width, float32(rows)*comboListRowHeight)
}

func (c *searchComboBox) pick(idx int) {
	c.cancelClose()
	c.closeList()
	c.list.UnselectAll()
	c.ignoreChange = true
	if c.clearOnPick {
		c.entry.SetText("")
	} else if idx >= 0 && idx < len(c.options) {
		c.entry.SetText(c.options[idx].Label)
	}
	c.ignoreChange = false
	if c.onPick != nil {
		c.onPick(idx)
	}
}
