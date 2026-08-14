package ui

import (
	"fmt"
	"time"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/store"
)

// numberer is the subset of *store.DB that CloneInvoice needs, so it can be
// tested against a fake without a real SQLite database.
type numberer interface {
	NextInvoiceNumber(pattern string, date time.Time) (string, error)
}

var _ numberer = (*store.DB)(nil)

// CloneInvoice is the primary recurring-invoice flow (per the plan: "Clone
// copies everything, allocates the next number in that series, and sets
// today's date"). It deep-copies inv's content, allocates the next number
// in the series for pattern via db, and stamps today's date, returning a
// new unsaved invoice (ID reset to 0) ready to review/edit before saving.
func CloneInvoice(db numberer, pattern string, inv model.Invoice, today time.Time) (model.Invoice, error) {
	if pattern == "" {
		return model.Invoice{}, fmt.Errorf("clone: no numbering pattern configured for invoice type %q", inv.Type)
	}

	clone := inv
	clone.ID = 0
	clone.Date = today

	number, err := db.NextInvoiceNumber(pattern, today)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("clone: allocate number: %w", err)
	}
	clone.Number = number

	// Deep-copy slice/pointer fields so mutating the clone in the editor
	// never aliases the original invoice's backing arrays.
	clone.Items = append([]model.LineItem(nil), inv.Items...)
	clone.Customer.BillingAddress = append([]string(nil), inv.Customer.BillingAddress...)
	clone.Customer.ShippingAddress = append([]string(nil), inv.Customer.ShippingAddress...)
	clone.Seller.AddressLines = append([]string(nil), inv.Seller.AddressLines...)
	if inv.ShippingBillDate != nil {
		d := *inv.ShippingBillDate
		clone.ShippingBillDate = &d
	}

	return clone, nil
}

// EditorStateFromInvoice converts a saved model.Invoice into editable
// (string-typed) EditorState, e.g. for the Edit and Clone flows.
func EditorStateFromInvoice(inv model.Invoice) *EditorState {
	s := &EditorState{
		ID:               inv.ID,
		Type:             inv.Type,
		Number:           inv.Number,
		ReferenceNo:      inv.ReferenceNo,
		Date:             inv.Date,
		DueDate:          inv.DueDate,
		CopyType:         inv.CopyType,
		Customer:         inv.Customer,
		PlaceOfSupply:    inv.PlaceOfSupply,
		CountryOfSupply:  inv.CountryOfSupply,
		Currency:         inv.Currency,
		FXFactor:         inv.FXFactor.String(),
		ShippingBillNo:   inv.ShippingBillNo,
		ShippingPortCode: inv.ShippingPortCode,
		Notes:            inv.Notes,
		Seller:           inv.Seller,
		Bank:             inv.Bank,
		// Preserve empty LayoutStyle so invoices that never had Layout set
		// in the editor keep following Settings (see EffectiveLayoutStyle).
		LayoutStyle: inv.LayoutStyle,
	}
	if inv.ShippingBillDate != nil {
		s.ShippingBillDate = inv.ShippingBillDate.Format("2006-01-02")
	}
	for _, it := range inv.Items {
		s.Items = append(s.Items, &LineItemState{
			Description: it.Description,
			HSNSAC:      it.HSNSAC,
			Quantity:    it.Quantity.String(),
			Unit:        it.Unit,
			RateUSD:     amountToDecimalString(it.RateUSD),
			DiscountUSD: amountToDecimalString(it.DiscountUSD),
			TaxRatePct:  bpsToPercentString(it.TaxRatePct),
			CessRatePct: bpsToPercentString(it.CessRatePct),
		})
	}
	if len(s.Items) == 0 {
		s.Items = []*LineItemState{NewLineItemState("", "", 0)}
	}
	return s
}

// amountToDecimalString renders a money.Amount (minor units) as a plain
// decimal string ("4000.00") suitable for an editable text field — no
// digit grouping, unlike money.FormatUSD/FormatINR which are display-only.
func amountToDecimalString(a money.Amount) string {
	v := int64(a)
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d.%02d", v/100, v%100)
	if neg {
		s = "-" + s
	}
	return s
}
