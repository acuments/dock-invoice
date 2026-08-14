// Package ui implements the Fyne desktop UI: four screens (Invoices,
// Invoice editor, Settings, Masters) over the same internal/calc,
// internal/model and internal/store packages the PDF renderer uses.
//
// The logic-bearing parts of the editor (building/validating an invoice
// from in-progress field text, recomputing totals, and the invoice-type
// visibility rules) are deliberately kept free of any Fyne widget
// dependency in this file so they can be unit tested directly. editor.go
// binds Fyne widgets on top of this state.
package ui

import (
	"fmt"
	"strings"
	"time"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

// LineItemState is the in-progress (string-typed) form of one line item
// row. Values are kept as raw text so a partially-typed field (e.g. "12."
// while the user is still typing "12.50") doesn't get clobbered by
// re-formatting on every keystroke; parsing only happens in Build/Recalc.
type LineItemState struct {
	Description string
	HSNSAC      string
	Quantity    string
	Unit        string
	RateUSD     string
	DiscountUSD string
	TaxRatePct  string
	CessRatePct string
}

// NewLineItemState returns a line item pre-filled with the given defaults
// (typically internal/model.Defaults from Settings).
func NewLineItemState(defaultHSN, defaultUnit string, defaultTaxRatePctBps int32) *LineItemState {
	return &LineItemState{
		HSNSAC:      defaultHSN,
		Unit:        defaultUnit,
		Quantity:    "1.00",
		RateUSD:     "0.00",
		DiscountUSD: "0.00",
		TaxRatePct:  bpsToPercentString(defaultTaxRatePctBps),
		CessRatePct: "0",
	}
}

func bpsToPercentString(bps int32) string {
	whole := bps / 100
	frac := bps % 100
	if frac == 0 {
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d.%02d", whole, frac)
}

// build parses one line item's text fields into a model.LineItem.
func (li *LineItemState) build() (model.LineItem, error) {
	qty, err := money.ParseQty(orZero(li.Quantity))
	if err != nil {
		return model.LineItem{}, fmt.Errorf("quantity: %w", err)
	}
	rate, err := money.ParseRateDigits(orZero(li.RateUSD), 2)
	if err != nil {
		return model.LineItem{}, fmt.Errorf("rate: %w", err)
	}
	discount, err := money.ParseRateDigits(orZero(li.DiscountUSD), 2)
	if err != nil {
		return model.LineItem{}, fmt.Errorf("discount: %w", err)
	}
	taxBps, err := money.ParseRateDigits(orZero(li.TaxRatePct), 2)
	if err != nil {
		return model.LineItem{}, fmt.Errorf("tax rate: %w", err)
	}
	cessBps, err := money.ParseRateDigits(orZero(li.CessRatePct), 2)
	if err != nil {
		return model.LineItem{}, fmt.Errorf("cess rate: %w", err)
	}
	return model.LineItem{
		Description: li.Description,
		HSNSAC:      li.HSNSAC,
		Quantity:    qty,
		Unit:        li.Unit,
		RateUSD:     money.Amount(rate),
		DiscountUSD: money.Amount(discount),
		TaxRatePct:  int32(taxBps),
		CessRatePct: int32(cessBps),
	}, nil
}

func orZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
}

// EditorState is the full in-progress invoice being edited. All monetary /
// numeric fields are kept as raw text (see LineItemState) and only parsed
// in Build/Recalc, which is what a "recalc on every keystroke" flow calls.
type EditorState struct {
	ID               int64
	Type             model.InvoiceType
	Number           string
	ReferenceNo      string
	Date             time.Time
	DueDate          time.Time
	CopyType         string
	Customer         model.Customer
	PlaceOfSupply    string
	CountryOfSupply  string
	Currency         string
	FXFactor         string
	ShippingBillNo   string
	ShippingBillDate string
	ShippingPortCode string
	Items            []*LineItemState
	Notes            string
	Seller           model.Company
	Bank             model.Bank
	LayoutStyle      model.LayoutStyle
}

// NewEditorState returns a blank editor state for a new export-under-LUT
// invoice (the most common case per the plan's sample), seeded from the
// seller/bank settings (including logo/signature path merging).
func NewEditorState(settings model.Settings) *EditorState {
	return &EditorState{
		Type:          model.InvoiceExportLUT,
		Date:          time.Now(),
		DueDate:       time.Now().AddDate(0, 0, settings.Defaults.PaymentTermDays),
		CopyType:      firstNonEmpty(settings.Defaults.CopyType, "ORIGINAL"),
		PlaceOfSupply: "97-Other Territory",
		Currency:      firstNonEmpty(settings.Defaults.Currency, "USD"),
		FXFactor:      firstNonEmpty(settings.LastFXFactor, "1"),
		Seller:        sellerFromSettings(settings),
		Bank:          settings.Bank,
		Items: []*LineItemState{
			NewLineItemState(settings.Defaults.HSNSAC, settings.Defaults.Unit, settings.Defaults.TaxRatePct),
		},
		// LayoutStyle stays empty until the user picks Layout in the editor.
		// An empty value means "follow Settings on every PDF export" (see
		// stampInvoiceFromSettings / model.EffectiveLayoutStyle).
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ShowShippingFields reports whether the shipping bill fields (no./date/
// port) should be shown — and are required — for the current invoice type.
// Per the plan, these only apply to the two export variants.
func (s *EditorState) ShowShippingFields() bool {
	return s.Type == model.InvoiceExportLUT || s.Type == model.InvoiceExportIGST
}

// ShippingRequired reports whether the shipping bill fields must be
// non-empty to save — true only for export_igst per the plan.
func (s *EditorState) ShippingRequired() bool {
	return s.Type == model.InvoiceExportIGST
}

// CurrencyLocked reports whether currency/FX-factor editing should be
// disabled in the UI (domestic invoices are always INR at factor 1).
func (s *EditorState) CurrencyLocked() bool {
	return s.Type == model.InvoiceDomestic
}

// ShowCrossBorderFields reports whether the country-of-supply and
// currency/FX-factor controls apply to this invoice.
//
// They are export concepts. A domestic supply is always INR at factor 1 (see
// CurrencyLocked) and has no country of supply — the place of supply, a state,
// is what identifies it. Leaving them on screen for domestic meant three dead
// controls: two greyed-out locked boxes and a country field whose example
// prompt suggested the United States.
func (s *EditorState) ShowCrossBorderFields() bool {
	return s.Type != model.InvoiceDomestic
}

// SetType updates the invoice type and applies the type-switch side
// effects described in the plan: domestic locks currency to INR and the
// factor to 1; switching away from domestic back to an export type
// restores USD (only if currency is still the locked INR value, so it
// doesn't clobber a manual override).
func (s *EditorState) SetType(t model.InvoiceType) {
	wasLocked := s.CurrencyLocked()
	s.Type = t
	if s.CurrencyLocked() {
		s.Currency = "INR"
		s.FXFactor = "1"
	} else if wasLocked {
		s.Currency = "USD"
	}
	if !s.ShowShippingFields() {
		s.ShippingBillNo = ""
		s.ShippingBillDate = ""
		s.ShippingPortCode = ""
	}
	// Same discipline as the shipping-bill fields above: clear what is no
	// longer shown, so a country typed on an export invoice cannot survive a
	// switch to domestic and print on a PDF whose form never displayed it.
	if !s.ShowCrossBorderFields() {
		s.CountryOfSupply = ""
	}
}

// AddLine appends a new blank line item.
func (s *EditorState) AddLine(defaultHSN, defaultUnit string, defaultTaxRatePctBps int32) {
	s.Items = append(s.Items, NewLineItemState(defaultHSN, defaultUnit, defaultTaxRatePctBps))
}

// ApplyCustomer snapshots a Masters customer onto the in-progress invoice.
// This is a paste of name/GSTIN/addresses, not a live reference — subsequent
// Masters edits never change an already-edited or already-saved invoice.
func (s *EditorState) ApplyCustomer(c model.Customer) {
	s.Customer = c
}

// AddLineFromItem appends a line item prefilled from a Masters saved-item
// template. defaultTaxRatePctBps comes from Settings (items don't store a
// tax rate). Like ApplyCustomer, this is a snapshot paste.
//
// The item's DefaultRate is priced in its own Currency, which need not
// match the invoice's — a domestic (INR) invoice can paste a USD-priced
// item, or an export (USD) invoice can paste an INR-priced one — so the
// rate is converted via rateForInvoiceCurrency before landing on the line.
func (s *EditorState) AddLineFromItem(it model.Item, defaultTaxRatePctBps int32) {
	s.Items = append(s.Items, &LineItemState{
		Description: it.Description,
		HSNSAC:      it.HSNSAC,
		Unit:        it.Unit,
		Quantity:    "1.00",
		RateUSD:     rateForInvoiceCurrency(it, s.Currency, s.FXFactor),
		DiscountUSD: "0.00",
		TaxRatePct:  bpsToPercentString(defaultTaxRatePctBps),
		CessRatePct: "0",
	})
}

// rateForInvoiceCurrency converts a saved item's DefaultRate (denominated
// in it.Currency) into a decimal string denominated in invoiceCurrency,
// via fxFactor ("INR per 1 USD" — the same factor the invoice's own totals
// use, see EditorState.FXFactor) when the two currencies differ.
//
// A blank/unparsable fxFactor (or a blank invoiceCurrency, e.g. a not-yet-
// built state) falls back to pasting the raw figure rather than blocking
// the picker — the rate field stays freely editable afterwards either way.
func rateForInvoiceCurrency(it model.Item, invoiceCurrency, fxFactor string) string {
	itemCurrency := firstNonEmpty(it.Currency, model.CurrencyUSD)
	if invoiceCurrency == "" || itemCurrency == invoiceCurrency {
		return amountToDecimalString(it.DefaultRate)
	}
	fx, err := money.ParseRate(orZero1(fxFactor))
	if err != nil || fx == 0 {
		return amountToDecimalString(it.DefaultRate)
	}
	switch {
	case itemCurrency == model.CurrencyUSD && invoiceCurrency == model.CurrencyINR:
		return amountToDecimalString(money.ConvertUSDToINR(it.DefaultRate, fx))
	case itemCurrency == model.CurrencyINR && invoiceCurrency == model.CurrencyUSD:
		return amountToDecimalString(money.ConvertINRToUSD(it.DefaultRate, fx))
	default:
		return amountToDecimalString(it.DefaultRate)
	}
}

// RemoveLine deletes the line item at idx, if present.
func (s *EditorState) RemoveLine(idx int) {
	if idx < 0 || idx >= len(s.Items) {
		return
	}
	s.Items = append(s.Items[:idx], s.Items[idx+1:]...)
}

// RemoveLineState deletes the given line item by identity. Preferred over
// RemoveLine from UI row-delete callbacks, since a row's position can shift
// after earlier rows are added/removed and a captured index would then
// delete the wrong row.
func (s *EditorState) RemoveLineState(li *LineItemState) {
	for i, x := range s.Items {
		if x == li {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			return
		}
	}
}

// Build parses all current field text into a model.Invoice. It never
// mutates s. Returns an error naming the first invalid field.
func (s *EditorState) Build() (*model.Invoice, error) {
	if len(s.Items) == 0 {
		return nil, fmt.Errorf("at least one line item is required")
	}
	fx, err := money.ParseRate(orZero1(s.FXFactor))
	if err != nil {
		return nil, fmt.Errorf("conversion factor: %w", err)
	}
	items := make([]model.LineItem, 0, len(s.Items))
	for i, li := range s.Items {
		m, err := li.build()
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		items = append(items, m)
	}

	var shipDate *time.Time
	if strings.TrimSpace(s.ShippingBillDate) != "" {
		t, err := time.Parse("2006-01-02", s.ShippingBillDate)
		if err != nil {
			return nil, fmt.Errorf("shipping bill date: %w", err)
		}
		shipDate = &t
	}

	return &model.Invoice{
		ID:               s.ID,
		Type:             s.Type,
		Number:           s.Number,
		ReferenceNo:      s.ReferenceNo,
		Date:             s.Date,
		DueDate:          s.DueDate,
		CopyType:         s.CopyType,
		Customer:         s.Customer,
		PlaceOfSupply:    s.PlaceOfSupply,
		CountryOfSupply:  s.CountryOfSupply,
		Currency:         s.Currency,
		FXFactor:         fx,
		ShippingBillNo:   s.ShippingBillNo,
		ShippingPortCode: s.ShippingPortCode,
		ShippingBillDate: shipDate,
		Items:            items,
		Notes:            s.Notes,
		Seller:           s.Seller,
		Bank:             s.Bank,
		LayoutStyle:      s.LayoutStyle,
	}, nil
}

func orZero1(s string) string {
	if strings.TrimSpace(s) == "" {
		return "1"
	}
	return s
}

// Recalc builds the invoice from current field text and computes totals
// via internal/calc — the single source of truth for tax logic. The UI
// layer must never duplicate this math; it only calls Recalc and displays
// the result.
func (s *EditorState) Recalc() (calc.InvoiceResult, error) {
	inv, err := s.Build()
	if err != nil {
		return calc.InvoiceResult{}, err
	}
	return calc.ComputeInvoice(inv), nil
}

// Validate returns a list of human-readable problems preventing save (empty
// when the state is ready to save). It includes both structural checks and
// the Build() parse error, if any, so the UI can show one unified list.
func (s *EditorState) Validate() []string {
	var errs []string

	if strings.TrimSpace(s.Customer.Name) == "" {
		errs = append(errs, "customer name is required")
	}
	if strings.TrimSpace(s.Number) == "" {
		errs = append(errs, "invoice number is required")
	}
	if len(s.Items) == 0 {
		errs = append(errs, "at least one line item is required")
	}
	if s.ShippingRequired() {
		if strings.TrimSpace(s.ShippingBillNo) == "" {
			errs = append(errs, "shipping bill no. is required for export with IGST")
		}
		if strings.TrimSpace(s.ShippingPortCode) == "" {
			errs = append(errs, "shipping port code is required for export with IGST")
		}
		if strings.TrimSpace(s.ShippingBillDate) == "" {
			errs = append(errs, "shipping bill date is required for export with IGST")
		}
	}

	if _, err := s.Build(); err != nil {
		errs = append(errs, err.Error())
	}

	return errs
}
