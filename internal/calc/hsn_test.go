package calc

import (
	"testing"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

func TestHSNSummary_GroupsByHSNAndRate(t *testing.T) {
	inv := &model.Invoice{
		Type:          model.InvoiceDomestic,
		PlaceOfSupply: "33-Tamil Nadu",
		FXFactor:      money.Rate(money.RateScale),
		Seller:        model.Company{StateCode: "33", GSTIN: "33AAAAA0000A1Z5"},
		Customer:      model.Customer{GSTIN: "33BBBBB1111B2Z6"},
		Items: []model.LineItem{
			{Description: "Dev", HSNSAC: "998314", Quantity: money.Qty(100), RateUSD: money.Amount(10000000), TaxRatePct: 1800},
			{Description: "Dev more", HSNSAC: "998314", Quantity: money.Qty(100), RateUSD: money.Amount(3500000), TaxRatePct: 1800},
			{Description: "Hosting", HSNSAC: "998315", Quantity: money.Qty(100), RateUSD: money.Amount(2000000), TaxRatePct: 1800},
			{Description: "Goods", HSNSAC: "998314", Quantity: money.Qty(100), RateUSD: money.Amount(1000000), TaxRatePct: 500},
		},
	}

	res := ComputeInvoice(inv)
	rows := HSNSummary(res)
	if len(rows) != 3 {
		t.Fatalf("HSNSummary rows = %d, want 3 (two rates of 998314 + one 998315)", len(rows))
	}

	// First appearance order: 998314@18%, 998315@18%, 998314@5%.
	if got, want := rows[0].HSNSAC, "998314"; got != want {
		t.Errorf("row0 HSN = %q, want %q", got, want)
	}
	if got, want := rows[0].TaxableINR, money.Amount(13500000); got != want {
		t.Errorf("row0 TaxableINR = %d, want %d (1,00,000 + 35,000)", got, want)
	}
	if got, want := rows[0].CGST, money.Amount(1215000); got != want {
		t.Errorf("row0 CGST = %d, want %d", got, want)
	}
	if got, want := rows[0].SGST, money.Amount(1215000); got != want {
		t.Errorf("row0 SGST = %d, want %d", got, want)
	}
	if got, want := rows[0].TotalTax, money.Amount(2430000); got != want {
		t.Errorf("row0 TotalTax = %d, want %d", got, want)
	}

	if got, want := rows[1].HSNSAC, "998315"; got != want {
		t.Errorf("row1 HSN = %q, want %q", got, want)
	}
	if got, want := rows[1].TaxableINR, money.Amount(2000000); got != want {
		t.Errorf("row1 TaxableINR = %d, want %d", got, want)
	}

	if got, want := rows[2].HSNSAC, "998314"; got != want {
		t.Errorf("row2 HSN = %q, want %q", got, want)
	}
	if got, want := rows[2].TaxRatePct, int32(500); got != want {
		t.Errorf("row2 TaxRatePct = %d, want %d", got, want)
	}
}

func TestHSNSummary_Empty(t *testing.T) {
	if rows := HSNSummary(InvoiceResult{}); rows != nil {
		t.Errorf("empty result = %v, want nil", rows)
	}
}

func TestPercentLabel(t *testing.T) {
	cases := []struct {
		bps  int32
		want string
	}{
		{1800, "18%"},
		{900, "9%"},
		{1250, "12.5%"},
		{0, "0%"},
	}
	for _, c := range cases {
		if got := PercentLabel(c.bps); got != c.want {
			t.Errorf("PercentLabel(%d) = %q, want %q", c.bps, got, c.want)
		}
	}
}
