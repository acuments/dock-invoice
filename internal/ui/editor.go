package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

const editorDateLayout = "2006-01-02"

// EditorMasters is the Masters data the editor can paste into an invoice
// (customers and saved item templates). Loaded once when the editor opens
// so mid-keystroke work isn't racing a concurrent Masters edit; reopening
// New/Edit/Clone refreshes the lists. Empty slices just leave the pickers
// with a no-data option — the free-text fields still work either way.
type EditorMasters struct {
	Customers []model.Customer
	Items     []model.Item
}

// Editor binds Fyne widgets on top of an EditorState. All tax/total math
// happens in EditorState.Recalc (which delegates to internal/calc); this
// type never computes a tax figure itself — it only displays what Recalc
// returns, from a single recalc() call wired to every field's OnChanged.
type Editor struct {
	state    *EditorState
	settings model.Settings
	masters  EditorMasters
	onSave   func(model.Invoice) error
	onCancel func()

	root fyne.CanvasObject

	typeSelect    *widget.Select
	shippingBox   fyne.CanvasObject
	shippingReq   *widget.Label
	sbNoEntry     *widget.Entry
	sbDateEntry   *widget.DateEntry
	sbPortEntry   *widget.Entry
	currencyEntry *widget.Entry
	fxEntry       *widget.Entry
	// The country-of-supply and currency/FX controls are export-only, so each
	// whole labelled field (caption, control and hint) is kept here to be
	// shown or hidden as a unit. supplySpacer takes over their share of the
	// row on domestic, so "Place of supply" keeps the same width either way
	// rather than stretching across the whole strip.
	posCombo      *searchComboBox
	countryEntry  *widget.Entry
	countryField  fyne.CanvasObject
	currencyField fyne.CanvasObject
	fxField       fyne.CanvasObject
	supplySpacer  fyne.CanvasObject
	// Customer free-text fields are kept on Editor (not local vars) so a
	// Masters customer pick can SetText them in sync with state; without
	// that, ApplyCustomer would update state only and leave the widgets
	// showing whatever the user previously typed.
	custNameEntry  *widget.Entry
	custGSTINEntry *widget.Entry
	billingEntry   *widget.Entry
	shippingEntry  *widget.Entry
	itemsBox       *fyne.Container
	// lineAmountLabels holds one label per row in itemsBox, in the same
	// order as e.state.Items, so recalc() can push each line's computed
	// amount (from internal/calc) back into its row. Rebuilt by refreshItems
	// whenever the rows themselves are rebuilt.
	lineAmountLabels []*widget.Label
	// totalsLabel keeps a markdown summary for tests and accessibility;
	// totalsPanel is the summary rail's ruled totals block.
	totalsLabel *widget.RichText
	totalsPanel *fyne.Container
	errorLabel  *widget.Label
	// typeBadge echoes the selected invoice type in the masthead, so the tax
	// treatment stays visible after the Type field scrolls away.
	typeBadge *fyne.Container
}

// NewEditor builds an Editor over state. masters is the current customer/
// saved-item lists (may be empty). onSave is called with the built invoice
// when the user clicks Save (after Validate() passes).
func NewEditor(state *EditorState, settings model.Settings, masters EditorMasters, onSave func(model.Invoice) error, onCancel func()) *Editor {
	e := &Editor{state: state, settings: settings, masters: masters, onSave: onSave, onCancel: onCancel}
	e.build()
	e.recalc()
	return e
}

// Container returns the editor's root canvas object.
func (e *Editor) Container() fyne.CanvasObject { return e.root }

func (e *Editor) build() {
	// ---- Type + numbering ----
	// Note: SetSelected fires OnChanged synchronously, and OnChanged calls
	// applyTypeVisibility(), which touches e.shippingBox/currencyEntry/
	// fxEntry — none of which exist yet at this point in build(). So the
	// initial selection is set at the very end of build(), once every
	// widget it might touch has been constructed.
	//
	// Options are human labels from labels.go (e.g. "Export (LUT)"), never
	// the raw serialisation values ("export_lut") — same rule as the
	// invoices table's Type column.
	e.typeSelect = widget.NewSelect(InvoiceTypeLabels(), func(v string) {
		if t, ok := InvoiceTypeFromLabel(v); ok {
			e.state.SetType(t)
			e.applyTypeVisibility()
			e.recalc()
		}
	})

	numberEntry := widget.NewEntry()
	numberEntry.SetPlaceHolder("AEX{FY}-{SEQ}")
	numberEntry.SetText(e.state.Number)
	numberEntry.OnChanged = func(v string) { e.state.Number = v }

	refEntry := widget.NewEntry()
	refEntry.SetPlaceHolder("Optional PO / reference")
	refEntry.SetText(e.state.ReferenceNo)
	refEntry.OnChanged = func(v string) { e.state.ReferenceNo = v }

	dateEntry := widget.NewDateEntry()
	d := e.state.Date
	dateEntry.SetDate(&d)
	dateEntry.OnChanged = func(t *time.Time) {
		if t != nil {
			e.state.Date = *t
		}
	}

	dueDateEntry := widget.NewDateEntry()
	dd := e.state.DueDate
	dueDateEntry.SetDate(&dd)
	dueDateEntry.OnChanged = func(t *time.Time) {
		if t != nil {
			e.state.DueDate = *t
		}
	}

	copyTypeSelect := widget.NewSelect([]string{"ORIGINAL", "DUPLICATE", "TRIPLICATE"}, func(v string) {
		e.state.CopyType = v
	})
	copyTypeSelect.SetSelected(firstNonEmpty(e.state.CopyType, "ORIGINAL"))

	// layoutSelect lets this one invoice override the Settings default PDF
	// layout (see EditorState.LayoutStyle / stampInvoiceFromSettings) — e.g.
	// keeping the colour Modern layout on an export invoice while the
	// Settings default has switched to Classic for domestic buyers.
	layoutSelect := widget.NewSelect([]string{
		model.LayoutStyleLabel(model.LayoutModern),
		model.LayoutStyleLabel(model.LayoutClassic),
	}, func(v string) {
		if ls, ok := model.LayoutStyleFromLabel(v); ok {
			e.state.LayoutStyle = ls
		}
	})
	layoutSelect.SetSelected(model.LayoutStyleLabel(model.EffectiveLayoutStyle(e.state.LayoutStyle, e.settings.LayoutStyle)))

	// ---- Customer ----
	e.custNameEntry = newRequiredEntry("Customer or company name")
	e.custNameEntry.SetText(e.state.Customer.Name)
	e.custNameEntry.OnChanged = func(v string) { e.state.Customer.Name = v; e.recalc() }

	e.custGSTINEntry = widget.NewEntry()
	e.custGSTINEntry.SetPlaceHolder("Optional for export")
	e.custGSTINEntry.SetText(e.state.Customer.GSTIN)
	e.custGSTINEntry.OnChanged = func(v string) { e.state.Customer.GSTIN = v }

	e.billingEntry = multiLineEntry(strings.Join(e.state.Customer.BillingAddress, "\n"), 3)
	e.billingEntry.SetPlaceHolder("Name, street, city, country…")
	e.billingEntry.OnChanged = func(v string) { e.state.Customer.BillingAddress = splitNonEmptyLines(v) }

	e.shippingEntry = multiLineEntry(strings.Join(e.state.Customer.ShippingAddress, "\n"), 3)
	e.shippingEntry.SetPlaceHolder("Ship-to address")
	e.shippingEntry.OnChanged = func(v string) { e.state.Customer.ShippingAddress = splitNonEmptyLines(v) }

	copyShipBtn := widget.NewButtonWithIcon("Same as bill-to", theme.ContentCopyIcon(), func() {
		e.shippingEntry.SetText(e.billingEntry.Text)
		e.state.Customer.ShippingAddress = splitNonEmptyLines(e.billingEntry.Text)
	})
	copyShipBtn.Importance = widget.LowImportance

	e.posCombo = e.buildPlaceOfSupplyPicker()

	e.countryEntry = widget.NewEntry()
	e.countryEntry.SetPlaceHolder("e.g. UNITED STATES OF AMERICA")
	e.countryEntry.SetText(e.state.CountryOfSupply)
	e.countryEntry.OnChanged = func(v string) { e.state.CountryOfSupply = v }

	customerPicker := e.buildCustomerPicker()

	// ---- Shipping bill (export only) ----
	// These three entries are kept on Editor (not local vars) because
	// applyTypeVisibility must be able to push state back into them: SetType
	// blanks the state fields when switching away from an export type, and
	// without a SetText/SetDate call here the widgets would keep showing
	// stale text that the state no longer has (see the B1 fix note below).
	e.sbNoEntry = widget.NewEntry()
	e.sbNoEntry.SetPlaceHolder("Shipping bill number")
	e.sbNoEntry.SetText(e.state.ShippingBillNo)
	e.sbNoEntry.OnChanged = func(v string) { e.state.ShippingBillNo = v }

	e.sbDateEntry = widget.NewDateEntry()
	e.syncShippingBillDateWidget()
	e.sbDateEntry.OnChanged = func(t *time.Time) {
		if t == nil {
			e.state.ShippingBillDate = ""
			return
		}
		e.state.ShippingBillDate = t.Format(editorDateLayout)
	}

	e.sbPortEntry = widget.NewEntry()
	e.sbPortEntry.SetPlaceHolder("e.g. INMAA1")
	e.sbPortEntry.SetText(e.state.ShippingPortCode)
	e.sbPortEntry.OnChanged = func(v string) { e.state.ShippingPortCode = v }

	e.shippingReq = widget.NewLabel("")
	e.shippingReq.Importance = widget.WarningImportance
	e.shippingReq.Wrapping = fyne.TextWrapWord
	// Built as a whole section so show/hide toggles the kicker, rule and
	// fields together — a headerless strip of inputs appearing mid-page on an
	// export type would read as a rendering glitch.
	e.shippingBox = fieldSectionWith("Shipping bill", "Required for export with IGST; optional for LUT.", nil,
		stack(spaceSm,
			fields([]float32{1.25, 1.0, 0.85},
				field("Bill no.", e.sbNoEntry),
				field("Bill date", e.sbDateEntry),
				field("Port code", e.sbPortEntry),
			),
			e.shippingReq,
		),
	)

	// ---- Currency / FX ----
	e.currencyEntry = widget.NewEntry()
	e.currencyEntry.SetPlaceHolder("USD")
	e.currencyEntry.SetText(e.state.Currency)
	e.currencyEntry.OnChanged = func(v string) { e.state.Currency = v; e.recalc() }

	e.fxEntry = newDecimalEntry("83.20")
	e.fxEntry.SetPlaceHolder("INR per 1 USD")
	e.fxEntry.SetText(e.state.FXFactor)
	e.fxEntry.OnChanged = func(v string) { e.state.FXFactor = v; e.recalc() }

	e.countryField = field("Country of supply", e.countryEntry)
	e.currencyField = field("Currency", e.currencyEntry)
	e.fxField = fieldWithHint("FX rate", e.fxEntry, "INR per 1 USD")
	e.supplySpacer = gap(0)

	// ---- Line items ----
	e.itemsBox = stack(spaceSm)
	e.refreshItems()
	addLineBtn := widget.NewButtonWithIcon("Add line", theme.ContentAddIcon(), func() {
		e.state.AddLine(e.settings.Defaults.HSNSAC, e.settings.Defaults.Unit, e.settings.Defaults.TaxRatePct)
		e.refreshItems()
		e.recalc()
	})
	addFromItemPicker := e.buildItemPicker()

	// ---- Totals ----
	// totalsLabel remains for tests (String() contains amounts); the visible
	// footer is a right-aligned document-style panel built in renderTotalsPanel.
	e.totalsLabel = widget.NewRichTextFromMarkdown("")
	e.totalsLabel.Hide()
	e.totalsPanel = stack(spaceXs)
	e.typeBadge = container.NewStack()
	e.errorLabel = widget.NewLabel("")
	e.errorLabel.Importance = widget.DangerImportance
	e.errorLabel.Wrapping = fyne.TextWrapWord
	e.errorLabel.Hide()

	saveBtn := widget.NewButtonWithIcon("Save & export PDF", theme.DocumentSaveIcon(), func() {
		errs := e.state.Validate()
		if len(errs) > 0 {
			e.setError("Cannot save:\n" + strings.Join(errs, "\n"))
			return
		}
		e.setError("")
		inv, err := e.state.Build()
		if err != nil {
			e.setError(err.Error())
			return
		}
		if e.onSave != nil {
			if err := e.onSave(*inv); err != nil {
				e.setError(err.Error())
			}
		}
	})
	saveBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", func() {
		if e.onCancel != nil {
			e.onCancel()
		}
	})

	// ── Invoice details ───────────────────────────────────────────────────
	// Copy and Layout are both "how this specific invoice prints" controls
	// (print-copy stamp vs. PDF template), so Layout joins Copy on the first
	// row rather than crowding the date/reference row below. Weights are
	// re-balanced for four columns instead of three: Type and Layout carry
	// the longest option labels ("Export (IGST)", "Classic (black & white)")
	// so they get the most room, while Invoice no. and Copy stay narrow.
	detailsSection := fieldSection("Invoice details", stack(spaceMd,
		fields([]float32{0.85, 1.15, 0.7, 1.2},
			field("Invoice no.", numberEntry),
			field("Type", e.typeSelect),
			field("Copy", copyTypeSelect),
			field("Layout", layoutSelect),
		),
		fields([]float32{1.1, 1.1, 1.4},
			field("Invoice date", dateEntry),
			field("Due date", dueDateEntry),
			field("Reference", refEntry),
		),
	))

	// ── Customer ──────────────────────────────────────────────────────────
	// Bill-to and ship-to captions share one row so the two address boxes
	// start on the same line. Previously the "Same as bill-to" button sat
	// inside the ship-to column, pushing that entry ~16px below its twin.
	addressCaptions := fields([]float32{1, 1},
		vcenter(captionText("Bill to")),
		container.New(&baselineTrailingLayout{}, captionText("Ship to"), copyShipBtn),
	)
	addressBoxes := fields([]float32{1, 1}, e.billingEntry, e.shippingEntry)

	customerSection := fieldSectionWith("Customer", "", fixedWidth(260, customerPicker), stack(spaceMd,
		fields([]float32{1.5, 1.0},
			field("Name", e.custNameEntry),
			fieldWithHint("GSTIN", e.custGSTINEntry, "First 2 digits set the buyer's state"),
		),
		stack(spaceXs, addressCaptions, addressBoxes),
		fields([]float32{1.2, 1.3, 0.6, 0.7, 2.6},
			fieldWithHint("Place of supply", e.posCombo, "GST state/UT. Type to search by name or code."),
			e.countryField,
			e.currencyField,
			e.fxField,
			e.supplySpacer,
		),
	))

	// ── Line items ────────────────────────────────────────────────────────
	// Column captions are printed once in a header band instead of being
	// repeated on every row — nine captions per line made a three-line
	// invoice read as a wall of labels rather than a table of figures.
	addLineBtn.Importance = widget.LowImportance
	linesToolbar := row(spaceSm, addLineBtn, fixedWidth(240, addFromItemPicker))
	linesSection := fieldSection("Line items", stack(spaceSm,
		lineItemsHeader(),
		e.itemsBox,
		gap(spaceXs),
		linesToolbar,
	))

	// ── The document sheet ────────────────────────────────────────────────
	// One page with ruled sections, rather than four floating cards: this is
	// an invoice, and it should look like a single sheet of paper.
	formSheet := sheet(pad(spaceXl, stack(spaceXl,
		detailsSection,
		customerSection,
		e.shippingBox,
		linesSection,
	)))

	masthead := padXY(spaceLg, 0, container.NewBorder(nil, nil, nil,
		container.NewCenter(e.typeBadge), titleText("Tax invoice"),
	))

	formColumn := container.NewVScroll(
		padXY(spaceLg, spaceLg, maxWidthCenter(editorFormMaxWidth, stack(spaceMd, masthead, formSheet))),
	)

	// ── Summary rail ──────────────────────────────────────────────────────
	// The grand total is the whole point of the screen, so it is pinned in
	// view rather than parked at the bottom of a long scroll. The save action
	// lives with the number it commits.
	saveBtn.Alignment = widget.ButtonAlignCenter
	rail := tintedSheet(pad(spaceLg, stack(spaceMd,
		kickerText("Summary"),
		rule(),
		e.totalsPanel,
		gap(spaceXs),
		e.errorLabel,
		container.NewVBox(saveBtn, cancelBtn),
	)))
	railColumn := fixedWidth(summaryRailWidth,
		container.NewVScroll(padXY(spaceLg, spaceLg, container.NewVBox(rail))),
	)

	e.root = container.NewBorder(nil, nil, nil, railColumn, formColumn)

	// Now that every widget applyTypeVisibility touches has been built,
	// it's safe to set the initial selection (which synchronously fires
	// OnChanged -> applyTypeVisibility) and/or apply visibility directly.
	e.typeSelect.SetSelected(InvoiceTypeLabel(e.state.Type))
	e.applyTypeVisibility()
}

// customerPickerPlaceholder is the customer combobox's empty-entry prompt.
const customerPickerPlaceholder = "Search saved customers…"

// itemPickerPlaceholder is the first option of the Masters saved-item
// select next to "Add line".
const itemPickerPlaceholder = "Choose a saved item…"

// buildCustomerPicker returns a type-to-filter combobox that pastes a Masters
// customer into the free-text fields, for the same reason buildItemPicker is
// one: a plain widget.Select means scrolling a fixed dropdown to find one
// entry, which stops being usable as soon as the customer list is longer than
// the screen. This searches by name and GSTIN as the user types instead.
//
// Picking is by index into e.masters.Customers, so two customers sharing a
// name need no disambiguated label to stay independently selectable — the
// GSTIN in each row's Detail is what tells them apart on screen.
func (e *Editor) buildCustomerPicker() *searchComboBox {
	return newSearchComboBox(customerPickerPlaceholder, "(no customers in Masters yet)", customerComboOptions(e.masters.Customers), func(idx int) {
		if idx < 0 || idx >= len(e.masters.Customers) {
			return
		}
		e.applyCustomerToForm(e.masters.Customers[idx])
	})
}

// buildItemPicker returns a type-to-filter combobox that appends a line
// item from a Masters saved-item template. A plain widget.Select forces
// scrolling a fixed dropdown to find one entry — impractical once a
// catalogue runs into the hundreds of SKUs — so this searches by
// description and HSN/SAC as the user types instead.
func (e *Editor) buildItemPicker() *searchComboBox {
	box := newSearchComboBox(itemPickerPlaceholder, "(no saved items in Masters yet)", itemComboOptions(e.masters.Items), func(idx int) {
		if idx < 0 || idx >= len(e.masters.Items) {
			return
		}
		e.state.AddLineFromItem(e.masters.Items[idx], e.settings.Defaults.TaxRatePct)
		e.refreshItems()
		e.recalc()
	})
	return box
}

const placeOfSupplyPlaceholder = "Search state or code"

func (e *Editor) buildPlaceOfSupplyPicker() *searchComboBox {
	opts := gstStateComboOptions()
	box := newSearchComboBox(placeOfSupplyPlaceholder, "(no GST states loaded)", opts, func(idx int) {
		if idx < 0 || idx >= len(opts) {
			return
		}
		e.state.PlaceOfSupply = opts[idx].Label
		e.recalc()
	})
	box.clearOnPick = false
	box.onTyped = func(q string) {
		e.state.PlaceOfSupply = q
		e.recalc()
	}
	box.setText(model.FormatGSTState(e.state.PlaceOfSupply))
	return box
}

func gstStateComboOptions() []comboOption {
	labels := model.GSTStateLabels()
	opts := make([]comboOption, len(labels))
	for i, label := range labels {
		opts[i] = comboOption{Label: label}
	}
	return opts
}

// applyCustomerToForm mirrors ApplyCustomer's state change into the customer
// Entry widgets — the same state/widget sync discipline applyTypeVisibility
// uses for shipping/currency fields.
func (e *Editor) applyCustomerToForm(c model.Customer) {
	e.state.ApplyCustomer(c)
	e.custNameEntry.SetText(c.Name)
	e.custGSTINEntry.SetText(c.GSTIN)
	e.billingEntry.SetText(strings.Join(c.BillingAddress, "\n"))
	e.shippingEntry.SetText(strings.Join(c.ShippingAddress, "\n"))
	e.recalc()
}

// customerComboOptions builds the searchComboBox option list for the Masters
// customers: Label is the name (what a user remembers and types), Detail is
// the GSTIN — shown in the dropdown row and matched while filtering, so
// typing a GSTIN finds a customer just as well as typing their name, and two
// customers sharing a name are still told apart on sight.
func customerComboOptions(customers []model.Customer) []comboOption {
	opts := make([]comboOption, len(customers))
	for i, c := range customers {
		label := c.Name
		if strings.TrimSpace(label) == "" {
			label = "(unnamed customer)"
		}
		opts[i] = comboOption{Label: label, Detail: firstNonEmpty(c.GSTIN, "no GSTIN")}
	}
	return opts
}

// itemComboOptions builds the searchComboBox option list for the Masters
// saved items: Label is the description (what a user most often remembers
// and types), Detail is the HSN/SAC plus formatted rate — both shown in the
// dropdown row and matched while filtering, so typing an HSN/SKU code finds
// an item just as well as typing its description.
func itemComboOptions(items []model.Item) []comboOption {
	opts := make([]comboOption, len(items))
	for i, it := range items {
		label := it.Description
		if strings.TrimSpace(label) == "" {
			label = "(unnamed item)"
		}
		hsn := firstNonEmpty(it.HSNSAC, "no HSN")
		opts[i] = comboOption{
			Label:  label,
			Detail: fmt.Sprintf("%s · %s", hsn, formatItemRate(it)),
		}
	}
	return opts
}

// Exported label text for the shipping-bill form items, kept close to the
// pdf package's label constants so the UI and PDF never drift apart, without
// creating an import cycle (internal/pdf does not depend on internal/ui).
const (
	LabelShippingBillNoField   = "Shipping Bill No."
	LabelShippingBillDateField = "Shipping Bill Date"
	LabelShippingPortCodeField = "Shipping Port Code"
)

// applyTypeVisibility shows/hides the shipping-bill section and enables/
// disables currency editing based on the current invoice type — the
// type-switch behaviour required by the plan.
//
// SetType (editorstate.go) is the source of truth for what happens to the
// underlying values on a type change: it forces Currency/FXFactor to
// "INR"/"1" when locking, restores "USD" when unlocking, and blanks the
// shipping-bill fields whenever they're no longer shown. This function must
// mirror every one of those value changes into the corresponding widget —
// not just toggle Show/Hide/Enable/Disable — otherwise the widget keeps
// displaying whatever the user last typed while the state has already moved
// on, and Build()/Validate() silently operate on different data than what's
// on screen (the B1 bug).
func (e *Editor) applyTypeVisibility() {
	e.refreshTypeBadge()
	if e.state.ShowShippingFields() {
		e.shippingBox.Show()
	} else {
		e.shippingBox.Hide()
	}
	if e.state.ShippingRequired() {
		e.shippingReq.SetText("Shipping bill fields are required for export with IGST.")
		e.shippingReq.Show()
	} else {
		// Hidden, not merely blanked: an empty widget.Label still occupies a
		// full line, which left a band of dead space above Line items on every
		// LUT invoice.
		e.shippingReq.SetText("")
		e.shippingReq.Hide()
	}
	// Always resync the shipping entries with state, even while hidden: once
	// ShowShippingFields() is false, SetType has already blanked these three
	// state fields, and leaving stale text in the (merely hidden) widgets
	// means it would silently reappear — looking filled-in but not matching
	// state — if the user switches back to an export type later.
	e.sbNoEntry.SetText(e.state.ShippingBillNo)
	e.syncShippingBillDateWidget()
	e.sbPortEntry.SetText(e.state.ShippingPortCode)
	// Same reason for country of supply: SetType blanks it when it stops
	// being shown, so mirror that into the widget rather than leaving text
	// that looks filled in but no longer matches state.
	e.countryEntry.SetText(e.state.CountryOfSupply)

	// Country of supply and the currency/FX pair are export concepts; on a
	// domestic invoice they are locked or meaningless, so the whole fields go
	// away and a spacer holds their place in the row.
	if e.state.ShowCrossBorderFields() {
		e.countryField.Show()
		e.currencyField.Show()
		e.fxField.Show()
		e.supplySpacer.Hide()
	} else {
		e.countryField.Hide()
		e.currencyField.Hide()
		e.fxField.Hide()
		e.supplySpacer.Show()
	}

	if e.state.CurrencyLocked() {
		e.currencyEntry.SetText(e.state.Currency)
		e.currencyEntry.Disable()
		e.fxEntry.SetText(e.state.FXFactor)
		e.fxEntry.Disable()
	} else {
		// Mirror the case above: unlocking doesn't just re-enable editing,
		// it also means SetType may have just changed Currency back to
		// "USD" — push that into the widget too instead of leaving whatever
		// text ("INR") was showing while the fields were locked/disabled.
		e.currencyEntry.SetText(e.state.Currency)
		e.currencyEntry.Enable()
		e.fxEntry.SetText(e.state.FXFactor)
		e.fxEntry.Enable()
	}
}

// syncShippingBillDateWidget pushes e.state.ShippingBillDate into the DateEntry.
// Shipping bill dates are stored as "YYYY-MM-DD" strings (optional field);
// the DateEntry itself displays in the locale format via SetDate.
func (e *Editor) syncShippingBillDateWidget() {
	if e.sbDateEntry == nil {
		return
	}
	raw := strings.TrimSpace(e.state.ShippingBillDate)
	if raw == "" {
		e.sbDateEntry.SetDate(nil)
		return
	}
	if t, err := time.Parse(editorDateLayout, raw); err == nil {
		e.sbDateEntry.SetDate(&t)
		return
	}
	e.sbDateEntry.SetDate(nil)
}

// refreshItems rebuilds the line-item cards from e.state.Items.
func (e *Editor) refreshItems() {
	e.itemsBox.RemoveAll()
	e.lineAmountLabels = e.lineAmountLabels[:0]
	for i, li := range e.state.Items {
		if i > 0 {
			e.itemsBox.Add(rule())
		}
		lineRow, amount := e.buildLineItemCard(li)
		e.lineAmountLabels = append(e.lineAmountLabels, amount)
		e.itemsBox.Add(lineRow)
	}
	e.itemsBox.Refresh()
}

// recalc is the single entry point that recomputes totals — wired to every
// field's OnChanged, per the plan's "re-render totals through a single
// recalc() call" guidance. It never computes tax itself; it only calls
// EditorState.Recalc (internal/calc) and renders the result.
func (e *Editor) recalc() {
	result, err := e.state.Recalc()
	if err != nil {
		e.totalsLabel.ParseMarkdown(fmt.Sprintf("*Totals unavailable: %s*", err.Error()))
		e.renderTotalsError(err.Error())
		for _, lbl := range e.lineAmountLabels {
			lbl.SetText("—")
		}
		return
	}
	// Keep markdown in totalsLabel so widget tests can still String()-match
	// amounts; the on-screen panel is the document-style footer.
	e.totalsLabel.ParseMarkdown(formatTotalsMarkdown(result, e.state.Currency))
	e.renderTotalsPanel(result, e.state.Currency)

	// result.Lines is produced by calc.ComputeInvoice iterating inv.Items in
	// order, and inv.Items is built from e.state.Items in order (see
	// EditorState.Build), which is also the order refreshItems used to
	// populate e.lineAmountLabels — so index i always refers to the same
	// line item in both slices.
	for i, lr := range result.Lines {
		if i >= len(e.lineAmountLabels) {
			break
		}
		e.lineAmountLabels[i].SetText("₹ " + money.FormatINR(lr.TotalINR))
	}
}

func formatTotalsMarkdown(r calc.InvoiceResult, currency string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**Taxable Amount:** ₹ %s\n\n", money.FormatINR(r.TaxableINR)))
	switch r.Mode {
	case calc.TaxCGSTSGST:
		b.WriteString(fmt.Sprintf("**CGST:** ₹ %s  **SGST:** ₹ %s\n\n", money.FormatINR(r.CGST), money.FormatINR(r.SGST)))
	default:
		b.WriteString(fmt.Sprintf("**IGST:** ₹ %s\n\n", money.FormatINR(r.IGST)))
	}
	if r.CESS != 0 {
		b.WriteString(fmt.Sprintf("**CESS:** ₹ %s\n\n", money.FormatINR(r.CESS)))
	}
	b.WriteString(fmt.Sprintf("**Total Tax:** ₹ %s\n\n", money.FormatINR(r.TotalTax)))
	if r.RoundOff != 0 {
		b.WriteString(fmt.Sprintf("**Round Off:** ₹ %s\n\n", money.FormatINR(r.RoundOff)))
	}
	b.WriteString(fmt.Sprintf("## Total Value: ₹ %s\n\n", money.FormatINR(r.GrandTotal)))
	if currency == "USD" {
		b.WriteString(fmt.Sprintf("**Total (USD):** %s\n", money.FormatUSD(r.TotalUSD)))
	}
	return b.String()
}

// totalsLine is one label + amount row in the summary rail. The amount is
// right-aligned so a column of figures lines up on its decimal tail — the
// embedded Noto Sans has no monospace companion, so a shared right edge is
// what does the aligning here.
func totalsLine(label, amount string) fyne.CanvasObject {
	l := text(label, textCaption, false, themeColor(theme.ColorNamePlaceHolder))
	a := amountText(amount, textCaption, false, themeColor(theme.ColorNameForeground))
	return container.NewBorder(nil, nil, l, nil, a)
}

// grandTotalRow is the "Total due" figure: the largest thing in the rail, set
// in ink rather than the accent. On a printed invoice the total is not a
// button — it earns its emphasis from size and the rule above it.
func grandTotalRow(label, amount string) fyne.CanvasObject {
	l := text(label, textCaption, true, themeColor(theme.ColorNameForeground))
	a := amountText(amount, textTitle, true, themeColor(theme.ColorNameForeground))
	return container.NewBorder(nil, nil, container.NewCenter(l), nil, a)
}

func (e *Editor) renderTotalsError(msg string) {
	if e.totalsPanel == nil {
		return
	}
	e.totalsPanel.RemoveAll()
	errLbl := widget.NewLabel("Totals unavailable: " + msg)
	errLbl.Importance = widget.DangerImportance
	errLbl.Wrapping = fyne.TextWrapWord
	e.totalsPanel.Add(errLbl)
	e.totalsPanel.Refresh()
}

// renderTotalsPanel rebuilds the summary rail's totals block with the
// hierarchy of a printed invoice foot: taxable → tax lines → ruled grand
// total → settlement currency.
func (e *Editor) renderTotalsPanel(r calc.InvoiceResult, currency string) {
	if e.totalsPanel == nil {
		return
	}
	e.totalsPanel.RemoveAll()
	e.totalsPanel.Add(totalsLine("Taxable amount", "₹ "+money.FormatINR(r.TaxableINR)))
	switch r.Mode {
	case calc.TaxCGSTSGST:
		e.totalsPanel.Add(totalsLine("CGST", "₹ "+money.FormatINR(r.CGST)))
		e.totalsPanel.Add(totalsLine("SGST", "₹ "+money.FormatINR(r.SGST)))
	default:
		e.totalsPanel.Add(totalsLine("IGST", "₹ "+money.FormatINR(r.IGST)))
	}
	if r.CESS != 0 {
		e.totalsPanel.Add(totalsLine("CESS", "₹ "+money.FormatINR(r.CESS)))
	}
	e.totalsPanel.Add(totalsLine("Total tax", "₹ "+money.FormatINR(r.TotalTax)))
	if r.RoundOff != 0 {
		e.totalsPanel.Add(totalsLine("Round off", "₹ "+money.FormatINR(r.RoundOff)))
	}
	e.totalsPanel.Add(gap(spaceXs))
	e.totalsPanel.Add(inkRule())
	e.totalsPanel.Add(gap(spaceXs))
	e.totalsPanel.Add(grandTotalRow("Total due", "₹ "+money.FormatINR(r.GrandTotal)))
	if currency == "USD" {
		e.totalsPanel.Add(totalsLine("Settled in USD", "$ "+money.FormatUSD(r.TotalUSD)))
	}
	e.totalsPanel.Refresh()
}

// setError shows or clears the rail's validation message. It is hidden rather
// than merely blanked when empty so it leaves no gap in the rail.
func (e *Editor) setError(msg string) {
	if e.errorLabel == nil {
		return
	}
	e.errorLabel.SetText(msg)
	if msg == "" {
		e.errorLabel.Hide()
		return
	}
	e.errorLabel.Show()
}

// refreshTypeBadge repaints the masthead badge for the current invoice type.
func (e *Editor) refreshTypeBadge() {
	if e.typeBadge == nil {
		return
	}
	e.typeBadge.RemoveAll()
	e.typeBadge.Add(badge(InvoiceTypeLabel(e.state.Type), invoiceTypeTone(e.state.Type)))
	e.typeBadge.Refresh()
}

// splitNonEmptyLines splits a multi-line address field on "\n" and keeps
// only the non-empty lines, wherever they fall — matching what the name
// promises. It previously only trimmed a trailing blank line, which left
// interior blank lines (e.g. from a pasted address with extra newlines, or
// the user hitting Enter twice) baked into billing/shipping/company
// addresses; those addresses are printed on the PDF one line per array
// entry, so a stray blank entry becomes a stray blank line on the invoice.
// An address has no use for a deliberate blank line the way, say, notes
// free text might, so dropping every empty line (not just a trailing one)
// is the behaviour actually wanted here — the fix is to make the function
// match its name rather than rename it to match its (buggy) behaviour.
// Shared as-is with settings.go (company address) and masters.go (customer
// billing/shipping address) — same signature, same "collapse blank lines"
// semantics is correct for all three callers.
func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
