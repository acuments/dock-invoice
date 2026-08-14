package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/numbering"
	"dock-invoice/internal/store"
)

var _ invoicesStore = (*store.DB)(nil)
var _ mastersStore = (*store.DB)(nil)

// invoicesStore is the subset of *store.DB the Invoices screen needs.
type invoicesStore interface {
	ListInvoices() ([]model.Invoice, error)
	SaveInvoice(model.Invoice) (int64, error)
	UpdateInvoice(model.Invoice) error
	DeleteInvoice(int64) error
	numberer
}

// invoiceListColumns is the column order: everything descriptive first, then
// the tax split, then the two money totals last. Figures belong on the row's
// right edge, where a shared right margin lines their digits up, and Type —
// a short tinted word — used to sit beyond them, which both broke that edge
// and left the type crowding the USD figure with one gap's clearance. Moved
// in front of the figures it also gives the eye a foothold halfway across a
// wide row.
//
// CGST/SGST/IGST sit ahead of INR Total in that order — the same order the
// printed invoice's own tax columns run in (see internal/pdf/table.go) — even
// though no single invoice ever carries all three at once (see calc.go:
// intra-state domestic charges CGST+SGST, everything else charges IGST or
// nothing). Splitting them into three columns rather than one "Tax" figure is
// what lets the footer band double as GSTR-1 source figures for a filtered
// month or FY.
var invoiceListColumns = []string{"Date", "Number", "Customer", "Type", "CGST", "SGST", "IGST", "INR Total", "USD Total"}

// Column indices, so the sort comparator and the cell writer refer to columns
// by name rather than by a bare number that silently means a different column
// if the order above ever changes again.
const (
	colDate = iota
	colNumber
	colCustomer
	colType
	colCGST
	colSGST
	colIGST
	colINRTotal
	colUSDTotal
)

// invoiceListWeights are proportional column widths shared by the header band
// and every row, so the columns stay aligned while adapting to the window
// instead of being pinned to fixed pixel widths. Customer gives up most of
// the width the three tax columns need — it is the one column whose content
// (a name) compresses gracefully via ellipsis, where a figure like
// "₹ 1,18,000.00" clipped mid-digit would be unreadable.
var invoiceListWeights = []float32{0.85, 1.05, 1.7, 0.85, 1.0, 1.0, 1.0, 1.2, 1.0}

// invoiceListColumnAlign fixes each column's alignment up front, and is
// re-applied on every row update: widget.List recycles row objects, so a
// property set for one invoice and not reset would leak into another.
var invoiceListColumnAlign = []fyne.TextAlign{
	fyne.TextAlignLeading,  // Date
	fyne.TextAlignLeading,  // Number
	fyne.TextAlignLeading,  // Customer
	fyne.TextAlignLeading,  // Type
	fyne.TextAlignTrailing, // CGST
	fyne.TextAlignTrailing, // SGST
	fyne.TextAlignTrailing, // IGST
	fyne.TextAlignTrailing, // INR Total
	fyne.TextAlignTrailing, // USD Total
}

// invoiceRowHeight gives each row enough air to read as a list entry rather
// than a spreadsheet cell.
const invoiceRowHeight float32 = 46

// invoicePageSize is how many filtered invoices each list page holds.
// Filtering and footer totals always run over the whole in-memory set
// (see applyFilter / syncSummary); only the viewport is sliced.
const invoicePageSize = 15

// invoiceListGutter is the horizontal inset from the sheet's edge to the first
// column's text, applied identically by the filter bar, the column-caption
// band, every row and the totals band — the list itself is not inset, so all
// four land on the same left edge.
const invoiceListGutter float32 = spaceLg

// allPeriodsLabel/allMonthsLabel are the period selects' "no filter" options.
// They sit first in each Select's Options and double as the sentinel text
// their OnChanged handlers check for to clear that half of the filter.
const (
	allPeriodsLabel = "All periods"
	allMonthsLabel  = "All months"
)

// invoiceMonthOptions is the month Select's fixed option list: unlike the FY
// list it never depends on the data, so it is built once rather than on
// every Reload.
var invoiceMonthOptions = func() []string {
	opts := make([]string, 0, 13)
	opts = append(opts, allMonthsLabel)
	for m := time.January; m <= time.December; m++ {
		opts = append(opts, m.String())
	}
	return opts
}()

// fyLabel renders a numbering.FY code ("2425") as the menu-friendly "FY
// 2024-25". The app has only ever produced invoices in the 2000s, so
// prefixing both halves with "20" is safe and keeps the label short — a
// four-digit year on each side of the dash would crowd a dropdown meant to
// be scanned at a glance.
func fyLabel(code string) string {
	if len(code) != 4 {
		return code
	}
	return "FY 20" + code[:2] + "-" + code[2:]
}

// fyCodeFromLabel inverts fyLabel, reading a numbering.FY code back off the
// Select's chosen text. allPeriodsLabel (and any unrecognised text) maps to
// "", clearing that half of the filter.
func fyCodeFromLabel(label string) string {
	if !strings.HasPrefix(label, "FY 20") {
		return ""
	}
	return strings.ReplaceAll(strings.TrimPrefix(label, "FY 20"), "-", "")
}

// monthFromLabel inverts invoiceMonthOptions' time.Month.String() labels.
// allMonthsLabel (and any unrecognised text) maps to 0, "every month".
func monthFromLabel(label string) time.Month {
	for m := time.January; m <= time.December; m++ {
		if m.String() == label {
			return m
		}
	}
	return 0
}

// InvoicesScreen is the app's landing screen: the invoice history table
// with New / Clone / Edit / Re-export PDF / Delete actions. Clone is the
// primary recurring-invoice flow (see clone.go).
type InvoicesScreen struct {
	win           fyne.Window
	db            invoicesStore
	getSettings   func() model.Settings
	onOpenEditor  func(state *EditorState)
	renderAndSave func(model.Invoice) (string, error)

	// allInvoices is every invoice the store returned by the last Reload.
	// invoices is the filtered+sorted full set used by the footer, sort, and
	// selection mapping. The list only renders one page of that slice
	// (see pageOffset / pageLength); s.selected is page-local.
	allInvoices []model.Invoice
	invoices    []model.Invoice
	filter      invoiceFilter
	selected    int
	// page is the 0-based page of s.invoices currently shown in the list.
	// pageSize defaults to invoicePageSize and is overridable in tests.
	page     int
	pageSize int
	// The list is a widget.List, not a widget.Table: Fyne separates table
	// cells with a hard padding gap, which meant a row's underline and the
	// header's tint broke into disconnected segments at every column boundary.
	// A List row is a single full-width object, so rules run edge to edge and
	// selection highlights the whole row natively.
	list        *widget.List
	headerCells []*canvas.Text
	empty       fyne.CanvasObject
	// noMatches is shown instead of empty when there are invoices but the
	// current filter excludes all of them — a distinct message from "no
	// invoices yet", since the fix here is "adjust the filter", not "create
	// one".
	noMatches fyne.CanvasObject
	status    *widget.Label
	root      fyne.CanvasObject
	// footer is the summary band under the list (see buildListFooter); it is
	// hidden along with the table itself while there is nothing to total.
	// pager sits above it and only shows once the filtered set spans more
	// than one page (see syncPagination).
	footer      fyne.CanvasObject
	pager       fyne.CanvasObject
	pageLabel   *canvas.Text
	prevPageBtn *widget.Button
	nextPageBtn *widget.Button
	footerCount *canvas.Text
	footerCGST  *canvas.Text
	footerSGST  *canvas.Text
	footerIGST  *canvas.Text
	footerINR   *canvas.Text
	footerUSD   *canvas.Text
	// Filter bar controls (see buildFilterBar); held on the screen so Reload
	// can rebuild the FY options and clearFilters can reset all three.
	searchEntry *widget.Entry
	fySelect    *widget.Select
	monthSelect *widget.Select
	clearBtn    *widget.Button
	exportBtn   *widget.Button
	// rowActions are the buttons that operate on the selected invoice; they
	// stay disabled until there is one (see syncActionState).
	rowActions []*widget.Button

	// sortCol/sortAsc record the user's chosen column sort, applied on top
	// of the store's own ordering by clicking a header cell (see
	// handleHeaderClick/applySort). sortCol == -1 means no override: the
	// list stays in the order ListInvoices returns it (most recently
	// created first), which is already the sensible default for "find the
	// invoice I just made".
	sortCol int
	sortAsc bool
}

// NewInvoicesScreen builds the Invoices screen.
//   - getSettings returns the current settings (for numbering patterns and
//     seeding a new invoice).
//   - onOpenEditor is called with a freshly built EditorState to switch the
//     app to the editor tab/window (New/Clone/Edit all route through this).
//   - renderAndSave renders an invoice's PDF and returns the output path.
func NewInvoicesScreen(win fyne.Window, db invoicesStore, getSettings func() model.Settings, onOpenEditor func(*EditorState), renderAndSave func(model.Invoice) (string, error)) *InvoicesScreen {
	s := &InvoicesScreen{win: win, db: db, getSettings: getSettings, onOpenEditor: onOpenEditor, renderAndSave: renderAndSave, selected: -1, sortCol: -1, pageSize: invoicePageSize}
	s.build()
	s.Reload()
	return s
}

func (s *InvoicesScreen) Container() fyne.CanvasObject { return s.root }

func (s *InvoicesScreen) build() {
	s.status = widget.NewLabel("")
	s.status.Importance = widget.LowImportance

	s.list = widget.NewList(
		func() int { return s.pageLength() },
		func() fyne.CanvasObject {
			return newInvoiceRow(
				func(id widget.ListItemID) { s.list.Select(id) },
				func(id widget.ListItemID) { s.editRow(id) },
			)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			invIdx := s.pageOffset() + int(id)
			if invIdx < 0 || invIdx >= len(s.invoices) {
				return
			}
			// The row still stores the list item id (page-local), not the
			// index into s.invoices: click/selection callbacks go through the
			// list, which only knows about the current page.
			o.(*invoiceRow).set(id, s.invoices[invIdx])
		},
	)
	s.list.OnSelected = func(id widget.ListItemID) {
		s.selected = int(id)
		s.syncActionState()
	}
	s.list.HideSeparators = true

	newBtn := widget.NewButtonWithIcon("New invoice", theme.ContentAddIcon(), func() {
		st := NewEditorState(s.getSettings())
		s.assignNumber(st)
		s.onOpenEditor(st)
	})
	newBtn.Importance = widget.HighImportance

	cloneBtn := widget.NewButtonWithIcon("Clone", theme.ContentCopyIcon(), func() { s.cloneSelected() })
	editBtn := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() { s.editSelected() })
	reexportBtn := widget.NewButton("Re-export PDF", func() { s.reexportSelected() })
	viewBtn := widget.NewButtonWithIcon("View PDF", theme.DocumentIcon(), func() { s.viewSelected() })
	deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() { s.deleteSelected() })
	deleteBtn.Importance = widget.DangerImportance

	// Every action below needs a selected row. Disabling them until there is
	// one both states the requirement up front — instead of answering a click
	// with "Select an invoice to…" — and stops a permanently-lit red Delete
	// from being the loudest thing on a screen whose primary job is New.
	s.rowActions = []*widget.Button{cloneBtn, editBtn, viewBtn, reexportBtn, deleteBtn}

	// New is the screen's purpose, so it sits in the masthead; the row actions
	// form a quieter secondary band beneath it.
	header := pageHeader(
		"Invoices",
		"Create, clone, and re-export GST tax invoices. Click a column header to sort, or double-click a row to edit it.",
		container.NewCenter(newBtn),
	)
	toolbar := padXY(0, spaceXl, container.NewBorder(nil, nil,
		row(spaceSm, cloneBtn, editBtn, viewBtn, reexportBtn), deleteBtn, nil,
	))

	s.empty = emptyState(
		"No invoices yet",
		"Create your first invoice, or clone a later one for recurring work.",
		func() fyne.CanvasObject {
			btn := widget.NewButtonWithIcon("New invoice", theme.ContentAddIcon(), func() {
				st := NewEditorState(s.getSettings())
				s.assignNumber(st)
				s.onOpenEditor(st)
			})
			btn.Importance = widget.HighImportance
			return btn
		}(),
	)
	s.noMatches = emptyState(
		"No matching invoices",
		"No invoice matches these filters. Try a different search or period.",
		widget.NewButton("Clear filters", func() { s.clearFilters() }),
	)

	// The filter bar is the outermost band docked to the sheet's top — it
	// owns the rounded corners bandFill protects — with the column-header
	// band now an interior strip beneath it (see buildListHeader).
	// The bottom of the sheet stacks the page controls above the totals
	// band: pagination only concerns the list viewport, while the footer
	// still totals every filtered invoice.
	tableHead := stack(0, s.buildFilterBar(), s.buildListHeader())
	tableFoot := stack(0, s.buildPaginationBar(), s.buildListFooter())
	listSurface := sheet(container.NewBorder(
		tableHead, tableFoot, nil, nil,
		// The list runs to the sheet's own edges — no inset — so a row's tint
		// spans the full width of the table it belongs to. Only the list takes
		// the flush-row theme; the two zero-data messages stacked with it are
		// ordinary widgets that still want the app's normal padding.
		container.NewStack(flushList(s.list), s.empty, s.noMatches),
	))

	s.root = container.NewBorder(
		stack(spaceMd, header, toolbar),
		statusBar(s.status),
		nil, nil,
		padXY(spaceXs, spaceXl, listSurface),
	)
	s.syncEmptyState()
	s.syncActionState()
	s.refreshListHeader()
}

// buildListHeader is the column band above the list. It lives outside the
// scrolling list rather than inside it as row zero, so it is always visible
// without needing to be pinned, and its rule spans the full sheet width.
//
// It sits below the filter bar (see buildFilterBar) rather than at the
// sheet's own top edge, so it no longer touches a rounded corner: a flat
// tinted rectangle stands in for bandFill, whose rounding and 1px inset exist
// solely to protect a corner this band no longer has.
func (s *InvoicesScreen) buildListHeader() fyne.CanvasObject {
	s.headerCells = make([]*canvas.Text, len(invoiceListColumns))
	cells := make([]fyne.CanvasObject, len(invoiceListColumns))
	for i := range invoiceListColumns {
		t := kickerText(invoiceListColumns[i])
		t.Alignment = invoiceListColumnAlign[i]
		s.headerCells[i] = t
		col := i
		cells[i] = newTappableArea(t, func() { s.handleHeaderClick(col) })
	}
	band := container.New(&ratioHBox{weights: invoiceListWeights, gap: spaceMd}, cells...)
	fill := canvas.NewRectangle(themeColor(theme.ColorNameHeaderBackground))
	return container.NewStack(fill, container.NewBorder(nil, rule(), nil, nil,
		padXY(spaceMd, invoiceListGutter, band),
	))
}

// buildFilterBar is the search + period band above the column-header band.
// Controls run left to right: free-text search (filling the remaining
// width), the FY select, the month select, and Clear. Entry.OnChanged is
// wired directly with no debounce — filtering a few thousand structs in
// memory is microseconds, so a debounce would only add latency the user can
// feel without saving any work the machine can't already do inline.
func (s *InvoicesScreen) buildFilterBar() fyne.CanvasObject {
	s.searchEntry = widget.NewEntry()
	s.searchEntry.PlaceHolder = "Search customer or invoice number"
	s.searchEntry.OnChanged = func(v string) {
		s.filter.query = v
		s.applyFilter()
	}

	s.fySelect = widget.NewSelect([]string{allPeriodsLabel}, func(label string) {
		s.filter.fy = fyCodeFromLabel(label)
		s.applyFilter()
	})
	s.fySelect.Selected = allPeriodsLabel

	s.monthSelect = widget.NewSelect(invoiceMonthOptions, func(label string) {
		s.filter.month = monthFromLabel(label)
		s.applyFilter()
	})
	s.monthSelect.Selected = allMonthsLabel

	// Clear only ever undoes an active filter, so it starts disabled rather
	// than sitting there as a no-op the first time the screen opens.
	s.clearBtn = widget.NewButton("Clear", func() { s.clearFilters() })
	s.clearBtn.Disable()

	s.exportBtn = widget.NewButtonWithIcon("Export CSV", theme.DownloadIcon(), func() { s.exportCSV() })
	s.exportBtn.Disable()

	bar := container.NewBorder(nil, nil, nil,
		row(spaceSm, s.fySelect, s.monthSelect, s.clearBtn, s.exportBtn),
		s.searchEntry,
	)
	return container.NewStack(bandFill(bandTop), container.NewBorder(nil, rule(), nil, nil,
		padXY(spaceMd, invoiceListGutter, bar),
	))
}

// buildPaginationBar is the Prev / page label / Next strip between the list
// and the totals footer. It is hidden while the filtered set fits on one
// page so a short history never grows chrome for navigation that cannot do
// anything.
func (s *InvoicesScreen) buildPaginationBar() fyne.CanvasObject {
	s.pageLabel = text("", textCaption, false, themeColor(theme.ColorNamePlaceHolder))
	s.pageLabel.Alignment = fyne.TextAlignCenter

	s.prevPageBtn = widget.NewButtonWithIcon("Previous", theme.NavigateBackIcon(), func() {
		s.changePage(-1)
	})
	s.nextPageBtn = widget.NewButtonWithIcon("Next", theme.NavigateNextIcon(), func() {
		s.changePage(1)
	})

	controls := container.NewBorder(nil, nil,
		s.prevPageBtn, s.nextPageBtn,
		container.NewCenter(s.pageLabel),
	)
	s.pager = container.NewBorder(nil, rule(), nil, nil,
		padXY(spaceSm, invoiceListGutter, controls),
	)
	s.pager.Hide()
	return s.pager
}

// buildListFooter is the summary band closing the sheet. It bookends the
// header, gives the white space below a short list a bottom edge instead of
// letting it fade out, and answers the two questions the table itself can
// only answer by adding rows up by eye: how many invoices are here, and what
// do they come to.
func (s *InvoicesScreen) buildListFooter() fyne.CanvasObject {
	s.footerCount = text("", textCaption, false, themeColor(theme.ColorNamePlaceHolder))
	s.footerCGST = text("", textCaption, true, themeColor(theme.ColorNameForeground))
	s.footerSGST = text("", textCaption, true, themeColor(theme.ColorNameForeground))
	s.footerIGST = text("", textCaption, true, themeColor(theme.ColorNameForeground))
	s.footerINR = text("", textCaption, true, themeColor(theme.ColorNameForeground))
	s.footerUSD = text("", textCaption, true, themeColor(theme.ColorNameForeground))

	// The totals ride the same column weights as the rows above, so each one
	// lands on the right edge of the column it totals rather than floating in
	// a separate summary line the eye has to match up by itself.
	cells := make([]fyne.CanvasObject, len(invoiceListColumns))
	for i := range cells {
		cells[i] = text("", textCaption, false, color.Transparent)
	}
	cells[colDate] = s.footerCount
	cells[colCGST] = s.footerCGST
	cells[colSGST] = s.footerSGST
	cells[colIGST] = s.footerIGST
	cells[colINRTotal] = s.footerINR
	cells[colUSDTotal] = s.footerUSD
	for _, c := range []*canvas.Text{s.footerCGST, s.footerSGST, s.footerIGST, s.footerINR, s.footerUSD} {
		c.Alignment = fyne.TextAlignTrailing
	}
	band := container.New(&ratioHBox{weights: invoiceListWeights, gap: spaceMd}, cells...)

	s.footer = container.NewStack(bandFill(bandBottom), container.NewBorder(rule(), nil, nil, nil,
		padXY(spaceMd, invoiceListGutter, band),
	))
	return s.footer
}

// syncSummary re-totals the footer over s.invoices — the filtered, displayed
// set, not s.allInvoices — since totalling to a chosen month is the whole
// point of filtering to it. The INR column sums every displayed invoice,
// since every invoice is raised in rupees; the USD, CGST, SGST and IGST
// columns each sum only over invoices that actually carry that figure (a
// dollar leg, or that tax), and stay blank rather than printing a "0.00" that
// would read as a real, if uneventful, total — the same reasoning the USD
// column already applied before CGST/SGST/IGST existed. This blank-when-zero
// behaviour is exactly what makes the CGST/SGST/IGST totals usable as
// GSTR-1 source figures for a filtered month or FY: a blank cell reads as
// "no domestic supply this period", not as a suspicious zero.
func (s *InvoicesScreen) syncSummary() {
	if s.footerCount == nil {
		return
	}

	var inr, usd, cgst, sgst, igst money.Amount
	for i := range s.invoices {
		result := calc.ComputeInvoice(&s.invoices[i])
		inr += result.GrandTotal
		cgst += result.CGST
		sgst += result.SGST
		igst += result.IGST
		if s.invoices[i].Currency == "USD" {
			usd += result.TotalUSD
		}
	}

	s.footerCount.Text = invoiceCountLabel(len(s.invoices), len(s.allInvoices), s.filter.active())
	s.footerCGST.Text = ""
	if cgst > 0 {
		s.footerCGST.Text = "₹ " + money.FormatINR(cgst)
	}
	s.footerSGST.Text = ""
	if sgst > 0 {
		s.footerSGST.Text = "₹ " + money.FormatINR(sgst)
	}
	s.footerIGST.Text = ""
	if igst > 0 {
		s.footerIGST.Text = "₹ " + money.FormatINR(igst)
	}
	s.footerINR.Text = "₹ " + money.FormatINR(inr)
	s.footerUSD.Text = ""
	if usd > 0 {
		s.footerUSD.Text = "$ " + money.FormatUSD(usd)
	}

	s.footerCount.Refresh()
	s.footerCGST.Refresh()
	s.footerSGST.Refresh()
	s.footerIGST.Refresh()
	s.footerINR.Refresh()
	s.footerUSD.Refresh()
}

// invoiceCountLabel is the footer's row-count caption: "412 invoices" (or
// "1 invoice") normally, or "34 of 412 invoices" once a filter has narrowed
// what is on screen — the total alongside the match count is what tells the
// user there was ever anything to filter, rather than reading as if the
// database only ever had 34 invoices in it. The plural in the filtered form
// tracks total, not shown, since the noun there names the pool being counted
// out of, not the (possibly singular, possibly zero) matches within it.
func invoiceCountLabel(shown, total int, filtered bool) string {
	if !filtered {
		if shown == 1 {
			return "1 invoice"
		}
		return fmt.Sprintf("%d invoices", shown)
	}
	noun := "invoices"
	if total == 1 {
		noun = "invoice"
	}
	return fmt.Sprintf("%d of %d %s", shown, total, noun)
}

// refreshListHeader re-renders the header captions, including the ▲/▼ marker
// on whichever column the list is currently sorted by.
func (s *InvoicesScreen) refreshListHeader() {
	for i, t := range s.headerCells {
		if t == nil {
			continue
		}
		t.Text = strings.ToUpper(s.headerLabel(i))
		if i == s.sortCol {
			t.Color = themeColor(theme.ColorNameForeground)
		} else {
			t.Color = themeColor(theme.ColorNamePlaceHolder)
		}
		t.Refresh()
	}
}

// syncActionState enables the row actions only while a row is selected.
func (s *InvoicesScreen) syncActionState() {
	has := s.selectedInvoice() != nil
	for _, b := range s.rowActions {
		if b == nil {
			continue
		}
		if has {
			b.Enable()
		} else {
			b.Disable()
		}
	}
}

// effectivePageSize is the page length the list math should use, falling
// back to the package default when pageSize is unset (a zero would
// otherwise hang pageCount on division by zero).
func (s *InvoicesScreen) effectivePageSize() int {
	if s.pageSize <= 0 {
		return invoicePageSize
	}
	return s.pageSize
}

// pageOffset is the inclusive start index of the current page in s.invoices.
func (s *InvoicesScreen) pageOffset() int {
	return s.page * s.effectivePageSize()
}

// pageLength is how many rows the list should report for the current page.
func (s *InvoicesScreen) pageLength() int {
	n := len(s.invoices) - s.pageOffset()
	if n <= 0 {
		return 0
	}
	if max := s.effectivePageSize(); n > max {
		return max
	}
	return n
}

// pageCount is the number of pages needed for the current filtered set.
// An empty set still counts as one page so "Page 1 of 1" style math never
// divides by zero; the pager itself is hidden when there is nothing to
// navigate (see syncPagination).
func (s *InvoicesScreen) pageCount() int {
	n := len(s.invoices)
	if n == 0 {
		return 1
	}
	size := s.effectivePageSize()
	return (n + size - 1) / size
}

// clampPage keeps s.page inside [0, pageCount-1] after a filter, sort, or
// reload shrinks the set under the page the user was on.
func (s *InvoicesScreen) clampPage() {
	max := s.pageCount() - 1
	if max < 0 {
		max = 0
	}
	if s.page < 0 {
		s.page = 0
	}
	if s.page > max {
		s.page = max
	}
}

// changePage steps the current page by delta (typically ±1), drops any
// selection that would now point at a different invoice, and refreshes the
// list. No-ops if the step would leave the valid range.
func (s *InvoicesScreen) changePage(delta int) {
	next := s.page + delta
	if next < 0 || next >= s.pageCount() {
		return
	}
	s.page = next
	// A list item id is page-local: keeping s.selected across a page flip
	// would leave Edit/Delete armed against whatever invoice happens to sit
	// at the same slot on the new page.
	s.selected = -1
	s.list.UnselectAll()
	s.list.Refresh()
	s.syncPagination()
	s.syncActionState()
}

// syncPagination refreshes the Prev/Next enablement, the "Page N of M /
// showing a–b" label, and whether the whole pager is needed. Totals stay
// on the full filtered set (syncSummary); this only describes the viewport.
func (s *InvoicesScreen) syncPagination() {
	if s.pager == nil {
		return
	}
	s.clampPage()
	pages := s.pageCount()
	n := len(s.invoices)
	// Hide when the set is empty or everything fits on one page — never
	// show permanently-disabled Prev/Next chrome.
	if n == 0 || pages <= 1 {
		s.pager.Hide()
		return
	}
	s.pager.Show()

	start := s.pageOffset() + 1
	end := s.pageOffset() + s.pageLength()
	s.pageLabel.Text = fmt.Sprintf("Page %d of %d · %d–%d", s.page+1, pages, start, end)
	s.pageLabel.Refresh()

	if s.page <= 0 {
		s.prevPageBtn.Disable()
	} else {
		s.prevPageBtn.Enable()
	}
	if s.page >= pages-1 {
		s.nextPageBtn.Disable()
	} else {
		s.nextPageBtn.Enable()
	}
}

// headerLabel returns the column-0 header text for col, with a small ▲/▼
// suffix appended to whichever column the list is currently sorted by, so
// the sort state is visible without a separate legend.
func (s *InvoicesScreen) headerLabel(col int) string {
	name := invoiceListColumns[col]
	if col != s.sortCol {
		return name
	}
	if s.sortAsc {
		return name + " ▲"
	}
	return name + " ▼"
}

// handleHeaderClick responds to a click on the header row (row 0 of the
// table). widget.Table has no dedicated header-click callback, and clicking
// row 0 through the normal OnSelected path would otherwise just leave a
// header cell stuck looking "selected" over no real data — so the click is
// repurposed as a sort control instead: clicking a column sorts by it,
// clicking the same column again reverses direction.
func (s *InvoicesScreen) handleHeaderClick(col int) {
	s.list.UnselectAll()
	if col == s.sortCol {
		s.sortAsc = !s.sortAsc
	} else {
		s.sortCol = col
		s.sortAsc = defaultSortAscending(col)
	}
	s.applySort()
	// Sorting reorders s.invoices, so any previous row index would now
	// point at a different invoice; drop the selection rather than risk
	// silently operating on the wrong one. Keep the page (clamped) so a
	// mid-list sort flip does not jump the viewport back to page 1.
	s.selected = -1
	s.clampPage()
	s.syncActionState()
	s.syncPagination()
	s.refreshListHeader()
	s.list.Refresh()
}

// defaultSortAscending picks a sensible initial direction the first time a
// column is clicked: newest/largest first for dates and amounts (matches
// "find my recent/biggest invoice"), alphabetical for text columns.
func defaultSortAscending(col int) bool {
	switch col {
	case colDate, colCGST, colSGST, colIGST, colINRTotal, colUSDTotal:
		return false
	default: // Number, Customer, Type
		return true
	}
}

// applySort reorders s.invoices in place per s.sortCol/s.sortAsc. sortCol
// == -1 (the initial state, and the state after every Reload) means "no
// user override": the store's own order is left untouched. ListInvoices
// returns rows most-recently-created first, which is already the sensible
// default for this screen's primary job — finding an invoice to clone.
func (s *InvoicesScreen) applySort() {
	if s.sortCol < 0 || len(s.invoices) < 2 {
		return
	}
	// Compute each row's derived totals once up front rather than calling
	// calc.ComputeInvoice repeatedly inside the comparator.
	inrTotal := make([]money.Amount, len(s.invoices))
	usdTotal := make([]money.Amount, len(s.invoices))
	cgstTotal := make([]money.Amount, len(s.invoices))
	sgstTotal := make([]money.Amount, len(s.invoices))
	igstTotal := make([]money.Amount, len(s.invoices))
	for i := range s.invoices {
		result := calc.ComputeInvoice(&s.invoices[i])
		inrTotal[i] = result.GrandTotal
		usdTotal[i] = result.TotalUSD
		cgstTotal[i] = result.CGST
		sgstTotal[i] = result.SGST
		igstTotal[i] = result.IGST
	}

	// less/idx are rebuilt as a permutation so both s.invoices and the
	// derived-total slices above can be reordered together in one pass.
	idx := make([]int, len(s.invoices))
	for i := range idx {
		idx[i] = i
	}
	less := func(a, b int) bool {
		i, j := idx[a], idx[b]
		var lt bool
		switch s.sortCol {
		case colDate:
			lt = s.invoices[i].Date.Before(s.invoices[j].Date)
		case colNumber:
			lt = s.invoices[i].Number < s.invoices[j].Number
		case colCustomer:
			lt = s.invoices[i].Customer.Name < s.invoices[j].Customer.Name
		case colType:
			lt = InvoiceTypeLabel(s.invoices[i].Type) < InvoiceTypeLabel(s.invoices[j].Type)
		case colCGST:
			lt = cgstTotal[i] < cgstTotal[j]
		case colSGST:
			lt = sgstTotal[i] < sgstTotal[j]
		case colIGST:
			lt = igstTotal[i] < igstTotal[j]
		case colINRTotal:
			lt = inrTotal[i] < inrTotal[j]
		case colUSDTotal:
			lt = usdTotal[i] < usdTotal[j]
		}
		if !s.sortAsc {
			return !lt
		}
		return lt
	}
	sort.SliceStable(idx, less)

	sorted := make([]model.Invoice, len(s.invoices))
	for newPos, oldPos := range idx {
		sorted[newPos] = s.invoices[oldPos]
	}
	s.invoices = sorted
}

func (s *InvoicesScreen) selectedInvoice() *model.Invoice {
	if s.selected < 0 || s.selected >= s.pageLength() {
		return nil
	}
	idx := s.pageOffset() + s.selected
	if idx < 0 || idx >= len(s.invoices) {
		return nil
	}
	return &s.invoices[idx]
}

// assignNumber allocates the next number for st's type using the
// configured pattern, so a brand-new invoice already has its number filled
// in before the editor opens.
func (s *InvoicesScreen) assignNumber(st *EditorState) {
	settings := s.getSettings()
	pattern := settings.NumberPatterns[st.Type]
	if pattern == "" {
		return
	}
	if n, err := s.db.NextInvoiceNumber(pattern, st.Date); err == nil {
		st.Number = n
	}
}

// editSelected opens the selected invoice in the editor (the toolbar's Edit).
func (s *InvoicesScreen) editSelected() {
	inv := s.selectedInvoice()
	if inv == nil {
		s.status.SetText("Select an invoice to edit.")
		return
	}
	s.onOpenEditor(EditorStateFromInvoice(*inv))
}

// editRow opens the invoice at a list index, for a double-clicked row. It
// selects the row first so the toolbar keeps pointing at the invoice the
// editor just opened, and re-checks the index rather than trusting it: rows
// are recycled, and a double-click arriving after a Reload could otherwise
// name a row that no longer exists. id is page-local (a widget.List item
// id), not an index into s.invoices.
func (s *InvoicesScreen) editRow(id widget.ListItemID) {
	if id < 0 || int(id) >= s.pageLength() {
		return
	}
	idx := s.pageOffset() + int(id)
	if idx < 0 || idx >= len(s.invoices) {
		return
	}
	s.list.Select(id)
	s.onOpenEditor(EditorStateFromInvoice(s.invoices[idx]))
}

func (s *InvoicesScreen) cloneSelected() {
	inv := s.selectedInvoice()
	if inv == nil {
		s.status.SetText("Select an invoice to clone.")
		return
	}
	settings := s.getSettings()
	pattern := settings.NumberPatterns[inv.Type]
	cloned, err := CloneInvoice(s.db, pattern, *inv, time.Now())
	if err != nil {
		s.status.SetText(fmt.Sprintf("clone: %v", err))
		return
	}
	s.onOpenEditor(EditorStateFromInvoice(cloned))
}

func (s *InvoicesScreen) reexportSelected() {
	inv := s.selectedInvoice()
	if inv == nil {
		s.status.SetText("Select an invoice to re-export.")
		return
	}
	path, err := s.renderAndSave(*inv)
	if err != nil {
		s.status.SetText(fmt.Sprintf("export: %v", err))
		return
	}
	s.status.SetText("Exported: " + path)
}

// viewSelected renders (or re-renders) the selected invoice's PDF and opens
// it in the OS default viewer so the user can check the printed result
// without hunting for the output folder.
func (s *InvoicesScreen) viewSelected() {
	inv := s.selectedInvoice()
	if inv == nil {
		s.status.SetText("Select an invoice to view.")
		return
	}
	path, err := s.renderAndSave(*inv)
	if err != nil {
		s.status.SetText(fmt.Sprintf("view: %v", err))
		return
	}
	if err := openFile(path); err != nil {
		s.status.SetText(fmt.Sprintf("open PDF: %v (saved at %s)", err, path))
		return
	}
	s.status.SetText("Opened: " + path)
}

// exportRows is the set exportCSV writes: every invoice the filters matched,
// in the list's sort order, snapshotted away from s.invoices. Split out from
// exportCSV so the scoping rule can be tested directly — the rest of that
// method is a file dialog, which a test cannot open, and a test that rebuilt
// this selection by hand would pass whatever the export actually did.
//
// The copy matters: the dialog's callback runs long after exportCSV returns,
// and s.invoices is replaced wholesale by every filter change and every
// Reload, so writing from the live field could export a set the user never
// saw — or race a reload mid-write.
func (s *InvoicesScreen) exportRows() []model.Invoice {
	rows := make([]model.Invoice, len(s.invoices))
	copy(rows, s.invoices)
	return rows
}

func (s *InvoicesScreen) exportCSV() {
	if len(s.invoices) == 0 {
		s.status.SetText("Nothing to export — no invoices match the current filters.")
		return
	}
	rows := s.exportRows()

	d := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
		// A nil writer with no error is the user cancelling; that is not a
		// failure worth a status line.
		if err != nil || wc == nil {
			return
		}
		path := wc.URI().Path()
		if err := writeInvoiceCSV(wc, rows); err != nil {
			wc.Close()
			s.status.SetText(fmt.Sprintf("export CSV: %v", err))
			return
		}
		if err := wc.Close(); err != nil {
			s.status.SetText(fmt.Sprintf("export CSV: %v", err))
			return
		}
		s.status.SetText(invoiceExportLabel(len(rows), path))
	}, s.win)
	d.SetFileName(invoiceCSVFilename(s.filter, time.Now()))
	d.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))
	d.Show()
}

func (s *InvoicesScreen) deleteSelected() {
	inv := s.selectedInvoice()
	if inv == nil {
		s.status.SetText("Select an invoice to delete.")
		return
	}
	dialog.ShowConfirm("Delete invoice", fmt.Sprintf("Delete invoice %s? This cannot be undone.", inv.Number), func(ok bool) {
		if !ok {
			return
		}
		if err := s.db.DeleteInvoice(inv.ID); err != nil {
			s.status.SetText(fmt.Sprintf("delete: %v", err))
			return
		}
		s.status.SetText("Deleted.")
		s.Reload()
	}, s.win)
}

// Reload refreshes the full invoice set from the database, then re-derives
// the FY filter's options and reapplies filter+sort (see applyFilter). Any
// user-chosen column sort (see applySort) is preserved across the reload;
// the selected row is not, since a row index from before the reload may now
// point at a different invoice (or none at all, if it was just deleted).
func (s *InvoicesScreen) Reload() {
	invs, err := s.db.ListInvoices()
	if err != nil {
		s.status.SetText(fmt.Sprintf("load invoices: %v", err))
		return
	}
	s.allInvoices = invs
	s.refreshFYOptions()
	s.applyFilter()
}

// applyFilter reruns s.filter over s.allInvoices, then whatever column sort
// is active, and refreshes everything downstream of s.invoices. It is the
// one place every filter control's OnChanged, Reload, and clearFilters all
// funnel through, so the "drop the stale selection" step below can't be
// forgotten on one of those paths the way a hand-rolled copy per caller
// could invite.
func (s *InvoicesScreen) applyFilter() {
	s.invoices = s.filter.apply(s.allInvoices)
	s.applySort()
	// Filtering changes which row index means which invoice just as
	// resorting does (see handleHeaderClick) — drop the selection rather
	// than risk Delete or Edit silently landing on a different invoice than
	// the one the user actually selected. Jump back to page 0 so the user
	// always lands at the top of the newly filtered result set rather than
	// on a page that may now be empty or past the end.
	s.selected = -1
	s.page = 0
	s.clampPage()
	s.list.UnselectAll()
	s.list.Refresh()
	s.syncEmptyState()
	s.syncActionState()
	if s.clearBtn != nil {
		if s.filter.active() {
			s.clearBtn.Enable()
		} else {
			s.clearBtn.Disable()
		}
	}
	if s.exportBtn != nil {
		if len(s.invoices) > 0 {
			s.exportBtn.Enable()
		} else {
			s.exportBtn.Disable()
		}
	}
}

// handleHeaderClick reorders s.invoices in place without re-running
// applyFilter. It drops selection like applyFilter does, but keeps the
// current page (clamped) so flipping a column sort does not yank the user
// back to page 1 mid-browse — unlike a filter change, which deliberately
// restarts at the top of the result set.

// clearFilters resets every filter control and re-derives the unfiltered
// list. The Select fields are written directly rather than through
// SetSelected, which would fire OnChanged and run applyFilter three times
// (once per control) before the query text was even cleared.
func (s *InvoicesScreen) clearFilters() {
	s.filter = invoiceFilter{}
	s.searchEntry.Text = ""
	s.searchEntry.Refresh()
	s.fySelect.Selected = allPeriodsLabel
	s.fySelect.Refresh()
	s.monthSelect.Selected = allMonthsLabel
	s.monthSelect.Refresh()
	s.applyFilter()
}

// refreshFYOptions rebuilds the FY select's options from s.allInvoices — a
// newly saved invoice can introduce a financial year that was not present
// before — and keeps the current selection if the FY it names still has
// invoices. If it doesn't (the last invoice in that FY was just deleted),
// the selection falls back to "All periods" rather than leave the dropdown
// pointing at a period that would now filter out every row.
func (s *InvoicesScreen) refreshFYOptions() {
	if s.fySelect == nil {
		return
	}
	opts := s.fyOptions()
	s.fySelect.SetOptions(opts)

	target := allPeriodsLabel
	if s.filter.fy != "" {
		if label := fyLabel(s.filter.fy); containsString(opts, label) {
			target = label
		} else {
			s.filter.fy = ""
		}
	}
	s.fySelect.Selected = target
	s.fySelect.Refresh()
}

// fyOptions lists allPeriodsLabel followed by one "FY 2024-25"-style label
// per financial year actually present in s.allInvoices, newest first. FY
// codes are two same-width digit pairs (see numbering.FY), so a plain string
// sort already orders them chronologically.
func (s *InvoicesScreen) fyOptions() []string {
	seen := make(map[string]bool)
	var codes []string
	for _, inv := range s.allInvoices {
		code := numbering.FY(inv.Date)
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(codes)))

	opts := make([]string, 0, len(codes)+1)
	opts = append(opts, allPeriodsLabel)
	for _, code := range codes {
		opts = append(opts, fyLabel(code))
	}
	return opts
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// syncEmptyState shows a calm zero-data message when there are no invoices
// at all, or — a distinct condition — a "no matches" message when there are
// invoices but the current filter excludes every one of them. The footer
// stays visible in the latter case (it is what tells you "0 of 412"), but
// hides along with the table when there is nothing in the database at all.
func (s *InvoicesScreen) syncEmptyState() {
	if s.empty == nil {
		return
	}
	switch {
	case len(s.allInvoices) == 0:
		s.empty.Show()
		s.noMatches.Hide()
		s.list.Hide()
		if s.footer != nil {
			s.footer.Hide()
		}
		if s.pager != nil {
			s.pager.Hide()
		}
	case len(s.invoices) == 0:
		s.empty.Hide()
		s.noMatches.Show()
		s.list.Hide()
		if s.footer != nil {
			s.footer.Show()
		}
		if s.pager != nil {
			s.pager.Hide()
		}
	default:
		s.empty.Hide()
		s.noMatches.Hide()
		s.list.Show()
		if s.footer != nil {
			s.footer.Show()
		}
	}
	s.syncSummary()
	s.syncPagination()
}

// SaveFromEditor saves inv: an insert for a new invoice (ID == 0, covering
// both "New" and "Clone", which always allocate a fresh number) or an
// update in place for "Edit" (ID != 0). Reloads the list afterwards. Used
// as the editor's onSave callback.
func (s *InvoicesScreen) SaveFromEditor(inv model.Invoice) error {
	if inv.ID == 0 {
		if _, err := s.db.SaveInvoice(inv); err != nil {
			return err
		}
	} else {
		if err := s.db.UpdateInvoice(inv); err != nil {
			return err
		}
	}
	s.Reload()
	return nil
}
