package ui

import (
	"testing"
	"time"

	"dock-invoice/internal/model"
)

func mkFilterInvoice(number, customer string, date time.Time) model.Invoice {
	return model.Invoice{Number: number, Customer: model.Customer{Name: customer}, Date: date}
}

func TestInvoiceFilter_QueryMatchesCustomerSubstring(t *testing.T) {
	inv := mkFilterInvoice("AEX-1", "Acme Corp", time.Now())
	f := invoiceFilter{query: "acme"}
	if !f.matches(inv) {
		t.Error("expected a substring of the customer name to match")
	}
	if (invoiceFilter{query: "zzz"}).matches(inv) {
		t.Error("a non-matching substring should not match")
	}
}

func TestInvoiceFilter_QueryMatchesNumberSubstring(t *testing.T) {
	inv := mkFilterInvoice("AEX2425-1234", "Northwind Trading Co.", time.Now())
	f := invoiceFilter{query: "1234"}
	if !f.matches(inv) {
		t.Error("expected a substring of the invoice number to match")
	}
}

func TestInvoiceFilter_QueryIsCaseInsensitive(t *testing.T) {
	inv := mkFilterInvoice("AEX-1", "Acme Corp", time.Now())
	for _, q := range []string{"ACME", "acme", "AcMe", "CORP"} {
		if !(invoiceFilter{query: q}).matches(inv) {
			t.Errorf("query %q should match %q case-insensitively", q, inv.Customer.Name)
		}
	}
}

// TestInvoiceFilter_QueryBlankIsNoFilter covers the "trim surrounding
// whitespace; an all-whitespace query is not a filter" rule: a search box a
// user tapped into and out of without typing anything real must not read as
// an active filter that excludes every row.
func TestInvoiceFilter_QueryBlankIsNoFilter(t *testing.T) {
	f := invoiceFilter{query: "   "}
	if f.active() {
		t.Error("an all-whitespace query should not count as active")
	}
	if !f.matches(mkFilterInvoice("A-1", "Anyone", time.Now())) {
		t.Error("an all-whitespace query should match everything")
	}
}

// TestInvoiceFilter_FYBoundary is the regression case for the Indian
// financial year's 1 April boundary: 15 Mar 2025 is the tail end of FY
// 2024-25, and 30 Apr 2025 — six weeks later — is already into FY 2025-26.
func TestInvoiceFilter_FYBoundary(t *testing.T) {
	tailOfFY2425 := mkFilterInvoice("A-1", "Cust", time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC))
	startOfFY2526 := mkFilterInvoice("A-2", "Cust", time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC))

	fy2425 := invoiceFilter{fy: "2425"}
	if !fy2425.matches(tailOfFY2425) {
		t.Error("15 Mar 2025 should match FY 2024-25")
	}
	if fy2425.matches(startOfFY2526) {
		t.Error("30 Apr 2025 should not match FY 2024-25")
	}

	fy2526 := invoiceFilter{fy: "2526"}
	if fy2526.matches(tailOfFY2425) {
		t.Error("15 Mar 2025 should not match FY 2025-26")
	}
	if !fy2526.matches(startOfFY2526) {
		t.Error("30 Apr 2025 should match FY 2025-26")
	}
}

func TestInvoiceFilter_Month(t *testing.T) {
	jan := mkFilterInvoice("A-1", "Cust", time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC))
	feb := mkFilterInvoice("A-2", "Cust", time.Date(2025, time.February, 10, 0, 0, 0, 0, time.UTC))

	f := invoiceFilter{month: time.January}
	if !f.matches(jan) {
		t.Error("a January invoice should match a January filter")
	}
	if f.matches(feb) {
		t.Error("a February invoice should not match a January filter")
	}
}

// TestInvoiceFilter_ConditionsAND checks that query/fy/month narrow the
// result together rather than any one of them being enough on its own.
func TestInvoiceFilter_ConditionsAND(t *testing.T) {
	inv := mkFilterInvoice("AEX-1", "Acme Corp", time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC))

	all := invoiceFilter{query: "acme", fy: "2425", month: time.January}
	if !all.matches(inv) {
		t.Fatal("a filter matching every field should match")
	}

	cases := []invoiceFilter{
		{query: "acme", fy: "2425", month: time.February}, // wrong month
		{query: "acme", fy: "2526", month: time.January},  // wrong FY
		{query: "zzz", fy: "2425", month: time.January},   // wrong query
	}
	for _, f := range cases {
		if f.matches(inv) {
			t.Errorf("filter %+v should not match when one condition disagrees", f)
		}
	}
}

func TestInvoiceFilter_Apply(t *testing.T) {
	invs := []model.Invoice{
		mkFilterInvoice("A-1", "Acme Corp", time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)),
		mkFilterInvoice("B-1", "Beta LLC", time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)),
	}

	got := (invoiceFilter{query: "acme"}).apply(invs)
	if len(got) != 1 || got[0].Number != "A-1" {
		t.Errorf("apply() = %v, want only A-1", got)
	}

	if got := (invoiceFilter{}).apply(invs); len(got) != len(invs) {
		t.Errorf("an inactive filter changed the count: got %d, want %d", len(got), len(invs))
	}
}
