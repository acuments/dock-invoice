package ui

import (
	"strings"
	"time"

	"dock-invoice/internal/model"
	"dock-invoice/internal/numbering"
)

// invoiceFilter is the invoices screen's search + period filter. There is no
// queryable date or customer column in the store — ListInvoices decodes every
// invoice from a JSON blob (see store/invoices.go) — so filtering always runs
// in memory over an already-loaded slice rather than becoming a WHERE clause.
// Kept as its own type, independent of InvoicesScreen, so the matching rules
// can be tested without building a screen at all.
type invoiceFilter struct {
	query string     // free text; "" (or all-whitespace) imposes no condition
	fy    string     // a numbering.FY code, e.g. "2425"; "" matches every FY
	month time.Month // 0 is not a real month, so it means "every month"
}

// active reports whether f would exclude anything, which is what decides
// whether the footer switches to "N of M" and whether Clear lights up.
func (f invoiceFilter) active() bool {
	return strings.TrimSpace(f.query) != "" || f.fy != "" || f.month != 0
}

// matches reports whether inv satisfies every condition f has set. Unset
// fields impose no condition, so multiple filters AND together rather than
// each narrowing the others out entirely.
func (f invoiceFilter) matches(inv model.Invoice) bool {
	if q := strings.ToLower(strings.TrimSpace(f.query)); q != "" {
		// Matched against both customer and number: a user chasing a
		// specific invoice down often remembers only one of the two, and
		// there is no way to tell up front which they typed.
		if !strings.Contains(strings.ToLower(inv.Customer.Name), q) &&
			!strings.Contains(strings.ToLower(inv.Number), q) {
			return false
		}
	}
	if f.fy != "" && numbering.FY(inv.Date) != f.fy {
		return false
	}
	if f.month != 0 && inv.Date.Month() != f.month {
		return false
	}
	return true
}

// apply returns the subset of invs that matches f, in the same order. It
// returns invs itself (not a copy) when f is inactive, since the caller's
// subsequent sort (see InvoicesScreen.applySort) always rebuilds a fresh
// slice rather than reordering in place, so aliasing the unfiltered slice
// here is safe.
func (f invoiceFilter) apply(invs []model.Invoice) []model.Invoice {
	if !f.active() {
		return invs
	}
	out := make([]model.Invoice, 0, len(invs))
	for _, inv := range invs {
		if f.matches(inv) {
			out = append(out, inv)
		}
	}
	return out
}
