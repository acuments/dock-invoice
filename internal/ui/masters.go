package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

// mastersStore is the subset of *store.DB the Masters screen needs.
type mastersStore interface {
	ListCustomers() ([]model.Customer, error)
	SaveCustomer(model.Customer) (int64, error)
	UpdateCustomer(model.Customer) error
	ListItems() ([]model.Item, error)
	SaveItem(model.Item) (int64, error)
	UpdateItem(model.Item) error
}

// MastersScreen manages the customer and saved-item master lists (lookup
// conveniences only — invoices snapshot their contents at save time, so
// editing a master here never changes an already-saved invoice).
type MastersScreen struct {
	win  fyne.Window
	db   mastersStore
	root fyne.CanvasObject

	customers     []model.Customer
	customerList  *widget.List
	customerEmpty fyne.CanvasObject // shown instead of the list when there are no customers yet
	selectedCust  int
	items         []model.Item
	itemList      *widget.List
	itemEmpty     fyne.CanvasObject // shown instead of the list when there are no saved items yet
	selectedItem  int
	status        *widget.Label

	// Edit acts on the selected row, so it stays disabled until there is one —
	// the same discipline the Invoices screen's row actions follow.
	editCustomerBtn *widget.Button
	editItemBtn     *widget.Button
}

func NewMastersScreen(win fyne.Window, db mastersStore) *MastersScreen {
	m := &MastersScreen{win: win, db: db}
	m.build()
	m.reloadCustomers()
	m.reloadItems()
	return m
}

func (m *MastersScreen) Container() fyne.CanvasObject { return m.root }

func (m *MastersScreen) build() {
	m.status = widget.NewLabel("")
	m.status.Importance = widget.LowImportance
	m.selectedCust = -1
	m.selectedItem = -1

	// Two-line list rows: primary name, muted secondary meta. Stack template
	// so UpdateItem can fill title + subtitle independently.
	m.customerList = widget.NewList(
		func() int { return len(m.customers) },
		masterRowTemplate,
		func(i widget.ListItemID, o fyne.CanvasObject) {
			c := m.customers[i]
			name := c.Name
			if strings.TrimSpace(name) == "" {
				name = "Unnamed customer"
			}
			gstin := firstNonEmpty(c.GSTIN, "No GSTIN")
			addrHint := ""
			if len(c.BillingAddress) > 0 {
				addrHint = " · " + c.BillingAddress[0]
			}
			setMasterRow(o, name, gstin+addrHint)
		},
	)
	m.customerList.HideSeparators = true
	m.customerList.OnSelected = func(id widget.ListItemID) {
		m.selectedCust = id
		m.syncActionState()
	}

	m.customerEmpty = emptyState(
		"No customers yet",
		"Save bill-to parties here once. The invoice editor can load them in one pick.",
		nil,
	)

	addCustomerBtn := widget.NewButtonWithIcon("Add customer", theme.ContentAddIcon(), func() {
		m.editCustomerDialog(nil)
	})
	addCustomerBtn.Importance = widget.HighImportance
	m.editCustomerBtn = widget.NewButton("Edit", func() {
		id := m.selectedCust
		if id < 0 || id >= len(m.customers) {
			m.status.SetText("Select a customer to edit.")
			return
		}
		c := m.customers[id]
		m.editCustomerDialog(&c)
	})
	customersPane := sheet(container.NewBorder(
		pad(spaceLg, stack(spaceXs,
			kickerText("Customers"),
			hintText("Snapshotted onto each invoice — later edits here never rewrite past PDFs."),
		)),
		pad(spaceLg, row(spaceSm, addCustomerBtn, m.editCustomerBtn)),
		nil, nil,
		container.NewStack(m.customerList, m.customerEmpty),
	))

	m.itemList = widget.NewList(
		func() int { return len(m.items) },
		masterRowTemplate,
		func(i widget.ListItemID, o fyne.CanvasObject) {
			it := m.items[i]
			desc := it.Description
			if strings.TrimSpace(desc) == "" {
				desc = "Unnamed item"
			}
			hsn := firstNonEmpty(it.HSNSAC, "—")
			unit := firstNonEmpty(it.Unit, "UNT")
			setMasterRow(o, desc, fmt.Sprintf("%s · %s · %s", hsn, unit, formatItemRate(it)))
		},
	)
	m.itemList.HideSeparators = true
	m.itemList.OnSelected = func(id widget.ListItemID) {
		m.selectedItem = id
		m.syncActionState()
	}

	m.itemEmpty = emptyState(
		"No saved items yet",
		"Templates for services and goods you invoice often. Add them here, pick them on the line.",
		nil,
	)

	addItemBtn := widget.NewButtonWithIcon("Add item", theme.ContentAddIcon(), func() {
		m.editItemDialog(nil)
	})
	addItemBtn.Importance = widget.HighImportance
	m.editItemBtn = widget.NewButton("Edit", func() {
		id := m.selectedItem
		if id < 0 || id >= len(m.items) {
			m.status.SetText("Select an item to edit.")
			return
		}
		it := m.items[id]
		m.editItemDialog(&it)
	})
	itemsPane := sheet(container.NewBorder(
		pad(spaceLg, stack(spaceXs,
			kickerText("Saved items"),
			hintText("Description, HSN/SAC, unit, and a default rate in INR or USD for fast line entry."),
		)),
		pad(spaceLg, row(spaceSm, addItemBtn, m.editItemBtn)),
		nil, nil,
		container.NewStack(m.itemList, m.itemEmpty),
	))

	header := pageHeader(
		"Masters",
		"Reusable customers and line templates. Invoices copy these values at save — history stays frozen.",
		nil,
	)

	m.root = container.NewBorder(
		header,
		statusBar(m.status),
		nil, nil,
		padXY(0, spaceXl, container.New(&ratioHBox{weights: []float32{1, 1}, gap: spaceLg},
			customersPane, itemsPane,
		)),
	)
	m.syncActionState()
}

// syncActionState enables each Edit button only while its list has a selection.
func (m *MastersScreen) syncActionState() {
	toggle := func(b *widget.Button, on bool) {
		if b == nil {
			return
		}
		if on {
			b.Enable()
		} else {
			b.Disable()
		}
	}
	toggle(m.editCustomerBtn, m.selectedCust >= 0 && m.selectedCust < len(m.customers))
	toggle(m.editItemBtn, m.selectedItem >= 0 && m.selectedItem < len(m.items))
}

func (m *MastersScreen) reloadCustomers() {
	cs, err := m.db.ListCustomers()
	if err != nil {
		m.status.SetText(fmt.Sprintf("load customers: %v", err))
		return
	}
	m.customers = cs
	// A row index from before this reload can no longer be trusted.
	m.selectedCust = -1
	m.customerList.UnselectAll()
	if len(m.customers) == 0 {
		m.customerEmpty.Show()
	} else {
		m.customerEmpty.Hide()
	}
	m.customerList.Refresh()
	m.syncActionState()
}

func (m *MastersScreen) reloadItems() {
	its, err := m.db.ListItems()
	if err != nil {
		m.status.SetText(fmt.Sprintf("load items: %v", err))
		return
	}
	m.items = its
	m.selectedItem = -1
	m.itemList.UnselectAll()
	if len(m.items) == 0 {
		m.itemEmpty.Show()
	} else {
		m.itemEmpty.Hide()
	}
	m.itemList.Refresh()
	m.syncActionState()
}

func (m *MastersScreen) editCustomerDialog(existing *model.Customer) {
	var c model.Customer
	if existing != nil {
		c = *existing
	}
	name := widget.NewEntry()
	name.SetText(c.Name)
	name.SetPlaceHolder("Customer or company name")
	gstin := widget.NewEntry()
	gstin.SetText(c.GSTIN)
	gstin.SetPlaceHolder("GSTIN — leave blank for export parties")
	billing := multiLineEntry(strings.Join(c.BillingAddress, "\n"), 4)
	billing.SetPlaceHolder("One line per address row")
	shipping := multiLineEntry(strings.Join(c.ShippingAddress, "\n"), 4)
	shipping.SetPlaceHolder("One line per address row")

	copyShip := widget.NewButton("Same as billing", func() {
		shipping.SetText(billing.Text)
	})
	copyShip.Importance = widget.LowImportance

	content := formStack(
		field("Name", name),
		fieldWithHint("GSTIN", gstin, "Optional for foreign customers"),
		field("Billing address", billing),
		stack(spaceXs,
			container.New(&baselineTrailingLayout{}, captionText("Shipping address"), copyShip),
			shipping,
		),
	)

	title := "New customer"
	if existing != nil {
		title = "Edit customer"
	}
	showSizedConfirm(m.win, title, pad(spaceLg, content), fyne.NewSize(560, 540), func(ok bool) {
		if !ok {
			return
		}
		c.Name = name.Text
		c.GSTIN = gstin.Text
		c.BillingAddress = splitNonEmptyLines(billing.Text)
		c.ShippingAddress = splitNonEmptyLines(shipping.Text)

		var err error
		if c.ID == 0 {
			_, err = m.db.SaveCustomer(c)
		} else {
			err = m.db.UpdateCustomer(c)
		}
		if err != nil {
			m.status.SetText(fmt.Sprintf("save customer: %v", err))
			return
		}
		m.status.SetText("Customer saved.")
		m.reloadCustomers()
	})
}

func (m *MastersScreen) editItemDialog(existing *model.Item) {
	var it model.Item
	if existing != nil {
		it = *existing
	}
	desc := widget.NewEntry()
	desc.SetText(it.Description)
	desc.SetPlaceHolder("e.g. Software Development Services")
	hsn := widget.NewEntry()
	hsn.SetText(it.HSNSAC)
	hsn.SetPlaceHolder("e.g. 998314")
	unit := widget.NewEntry()
	unit.SetText(it.Unit)
	unit.SetPlaceHolder("UNT")
	currency := widget.NewSelect([]string{model.CurrencyINR, model.CurrencyUSD}, nil)
	currency.SetSelected(firstNonEmpty(it.Currency, model.CurrencyUSD))
	rate := newDecimalEntry("0.00")
	rate.SetPlaceHolder("0.00")
	rate.SetText(amountToDecimalString(it.DefaultRate))

	content := formStack(
		field("Description", desc),
		fields([]float32{1.2, 0.7},
			field("HSN / SAC", hsn),
			field("Unit", unit),
		),
		fields([]float32{0.7, 1.3},
			field("Currency", currency),
			fieldWithHint("Default rate", rate, "Pasted onto new lines; adjustable per invoice"),
		),
	)

	title := "New saved item"
	if existing != nil {
		title = "Edit saved item"
	}
	showSizedConfirm(m.win, title, pad(spaceLg, content), fyne.NewSize(520, 420), func(ok bool) {
		if !ok {
			return
		}
		it.Description = desc.Text
		it.HSNSAC = hsn.Text
		it.Unit = unit.Text
		it.Currency = firstNonEmpty(currency.Selected, model.CurrencyUSD)
		if v, err := money.ParseRateDigits(orZero(rate.Text), 2); err == nil {
			it.DefaultRate = money.Amount(v)
		}

		var err error
		if it.ID == 0 {
			_, err = m.db.SaveItem(it)
		} else {
			err = m.db.UpdateItem(it)
		}
		if err != nil {
			m.status.SetText(fmt.Sprintf("save item: %v", err))
			return
		}
		m.status.SetText("Item saved.")
		m.reloadItems()
	})
}

// formatItemRate renders a saved item's default rate in its own stored
// currency (₹ Indian grouping for INR, $ Western grouping for USD) — the
// master no longer assumes every rate is USD, so the list/picker display
// has to say which currency each row actually means.
func formatItemRate(it model.Item) string {
	if it.Currency == model.CurrencyINR {
		return "₹" + money.FormatINR(it.DefaultRate)
	}
	return "$" + money.FormatUSD(it.DefaultRate)
}

// masterRowHeight keeps the two-line list rows compact; widget.List otherwise
// sizes them from a stack of padded Labels and they sprawl.
const masterRowHeight float32 = 52

// masterRowTemplate is the shared two-line row for both Masters lists.
//
// These rows used to be a VBox of widget.Labels, which carry ~10px of their
// own horizontal inner padding. That put the row text 6px to the left of the
// card's own heading — two competing left edges inside one card. Building the
// row from canvas.Text with an explicit inset makes it land on the same gutter
// as everything else in the card.
func masterRowTemplate() fyne.CanvasObject {
	title := text("", textBody, true, themeColor(theme.ColorNameForeground))
	sub := text("", textCaption, false, themeColor(theme.ColorNamePlaceHolder))
	sizer := canvas.NewRectangle(color.Transparent)
	sizer.SetMinSize(fyne.NewSize(1, masterRowHeight))
	return container.NewStack(sizer, container.NewBorder(nil, rule(), nil, nil,
		vcenter(padXY(0, spaceLg, stack(spaceXs, title, sub))),
	))
}

// setMasterRow fills a masterRowTemplate row.
func setMasterRow(o fyne.CanvasObject, title, sub string) {
	col := o.(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*fyne.Container)
	t := col.Objects[0].(*canvas.Text)
	s := col.Objects[1].(*canvas.Text)
	t.Text = title
	s.Text = sub
	t.Refresh()
	s.Refresh()
}
