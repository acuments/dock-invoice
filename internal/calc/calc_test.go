package calc

import (
	"testing"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

func mustRate(t *testing.T, s string) money.Rate {
	t.Helper()
	r, err := money.ParseRate(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestComputeInvoice_ExportLUT_Golden(t *testing.T) {
	inv := &model.Invoice{
		Type:          model.InvoiceExportLUT,
		PlaceOfSupply: "97-Other Territory",
		FXFactor:      mustRate(t, "83.2"),
		Seller:        model.Company{StateCode: "33"},
		Items: []model.LineItem{
			{
				Description: "Software Development Services",
				HSNSAC:      "998314",
				Quantity:    money.Qty(100), // 1.00
				Unit:        "UNT",
				RateUSD:     money.Amount(400000), // 4,000.00 USD
				DiscountUSD: 0,
				TaxRatePct:  1800, // @18%
			},
		},
	}

	res := ComputeInvoice(inv)

	if res.Mode != TaxIGST {
		t.Fatalf("Mode = %v, want TaxIGST", res.Mode)
	}
	if got, want := res.TaxableINR, money.Amount(33280000); got != want {
		t.Errorf("TaxableINR = %d, want %d", got, want)
	}
	if got, want := res.IGST, money.Amount(0); got != want {
		t.Errorf("IGST = %d, want %d (LUT must be zero)", got, want)
	}
	if got, want := res.TotalTax, money.Amount(0); got != want {
		t.Errorf("TotalTax = %d, want %d", got, want)
	}
	if got, want := res.GrandTotal, money.Amount(33280000); got != want {
		t.Errorf("GrandTotal = %d, want %d", got, want)
	}
	if got, want := res.TotalUSD, money.Amount(400000); got != want {
		t.Errorf("TotalUSD = %d, want %d", got, want)
	}
	if got, want := RateLabel(res.Lines[0].Line.TaxRatePct), "@18%"; got != want {
		t.Errorf("RateLabel = %q, want %q", got, want)
	}
	if res.RoundOff != 0 {
		t.Errorf("RoundOff = %d, want 0 for export", res.RoundOff)
	}
}

func TestComputeInvoice_ExportIGST(t *testing.T) {
	inv := &model.Invoice{
		Type:          model.InvoiceExportIGST,
		PlaceOfSupply: "97-Other Territory",
		FXFactor:      mustRate(t, "83.2"),
		Seller:        model.Company{StateCode: "33"},
		Items: []model.LineItem{
			{
				Quantity:   money.Qty(100),
				RateUSD:    money.Amount(400000),
				TaxRatePct: 1800,
			},
		},
	}
	res := ComputeInvoice(inv)
	if res.Mode != TaxIGST {
		t.Fatalf("Mode = %v, want TaxIGST", res.Mode)
	}
	// 18% of 3,32,800.00 = 59,904.00
	if got, want := res.IGST, money.Amount(5990400); got != want {
		t.Errorf("IGST = %d, want %d", got, want)
	}
	if got, want := res.GrandTotal, money.Amount(33280000+5990400); got != want {
		t.Errorf("GrandTotal = %d, want %d", got, want)
	}
}

func TestComputeInvoice_DomesticIntraState(t *testing.T) {
	inv := &model.Invoice{
		Type:          model.InvoiceDomestic,
		PlaceOfSupply: "33-Tamil Nadu",
		FXFactor:      money.Rate(money.RateScale), // 1.0 for domestic
		Seller:        model.Company{StateCode: "33"},
		Items: []model.LineItem{
			{
				Quantity:   money.Qty(100),
				RateUSD:    money.Amount(1000000), // treated as INR paise equiv when factor=1
				TaxRatePct: 1800,
			},
		},
	}
	res := ComputeInvoice(inv)
	if res.Mode != TaxCGSTSGST {
		t.Fatalf("Mode = %v, want TaxCGSTSGST", res.Mode)
	}
	// 9% CGST + 9% SGST on 10,000.00 = 900.00 each
	if got, want := res.CGST, money.Amount(90000); got != want {
		t.Errorf("CGST = %d, want %d", got, want)
	}
	if got, want := res.SGST, money.Amount(90000); got != want {
		t.Errorf("SGST = %d, want %d", got, want)
	}
	if res.IGST != 0 {
		t.Errorf("IGST = %d, want 0 for intra-state", res.IGST)
	}
}

func TestComputeInvoice_DomesticInterState(t *testing.T) {
	inv := &model.Invoice{
		Type:          model.InvoiceDomestic,
		PlaceOfSupply: "27-Maharashtra",
		FXFactor:      money.Rate(money.RateScale),
		Seller:        model.Company{StateCode: "33"},
		Items: []model.LineItem{
			{
				Quantity:   money.Qty(100),
				RateUSD:    money.Amount(1000000),
				TaxRatePct: 1800,
			},
		},
	}
	res := ComputeInvoice(inv)
	if res.Mode != TaxIGST {
		t.Fatalf("Mode = %v, want TaxIGST for inter-state", res.Mode)
	}
	if got, want := res.IGST, money.Amount(180000); got != want {
		t.Errorf("IGST = %d, want %d", got, want)
	}
	if res.CGST != 0 || res.SGST != 0 {
		t.Errorf("CGST/SGST should be 0 for inter-state, got %d/%d", res.CGST, res.SGST)
	}
}

// ---- GSTIN-driven intra-state detection -----------------------------------

// domesticInvoice builds a one-line domestic invoice at 18% on 10,000.00, so
// the tests below differ only in the two things that decide the split.
func domesticInvoice(sellerGSTIN, sellerStateCode, customerGSTIN, placeOfSupply string) *model.Invoice {
	return &model.Invoice{
		Type:          model.InvoiceDomestic,
		PlaceOfSupply: placeOfSupply,
		FXFactor:      money.Rate(money.RateScale),
		Seller:        model.Company{GSTIN: sellerGSTIN, StateCode: sellerStateCode},
		Customer:      model.Customer{GSTIN: customerGSTIN},
		Items: []model.LineItem{
			{Quantity: money.Qty(100), RateUSD: money.Amount(1000000), TaxRatePct: 1800},
		},
	}
}

// TestComputeInvoice_DomesticSplitFromGSTINStateCodes is the core of the
// rule: the state of each party comes from the first two digits of its GSTIN,
// and a match means an equal CGST/SGST split rather than IGST.
func TestComputeInvoice_DomesticSplitFromGSTINStateCodes(t *testing.T) {
	cases := []struct {
		name                                       string
		sellerGSTIN, sellerState, custGSTIN, place string
		wantMode                                   TaxMode
	}{
		{
			name:        "same state from both GSTINs",
			sellerGSTIN: "33AAECA1234A1Z9", custGSTIN: "33AAAAA0000A1Z5",
			place: "33-Tamil Nadu", wantMode: TaxCGSTSGST,
		},
		{
			name:        "different states from both GSTINs",
			sellerGSTIN: "33AAECA1234A1Z9", custGSTIN: "27BBBBB1111B2Z6",
			place: "27-Maharashtra", wantMode: TaxIGST,
		},
		{
			// A registered buyer's GSTIN wins over a place of supply that
			// disagrees with it.
			name:        "GSTIN state beats a conflicting place of supply",
			sellerGSTIN: "33AAECA1234A1Z9", custGSTIN: "33AAAAA0000A1Z5",
			place: "27-Maharashtra", wantMode: TaxCGSTSGST,
		},
		{
			name:        "unregistered buyer falls back to place of supply",
			sellerGSTIN: "33AAECA1234A1Z9", custGSTIN: "",
			place: "33-Tamil Nadu", wantMode: TaxCGSTSGST,
		},
		{
			name:        "unregistered buyer in another state stays IGST",
			sellerGSTIN: "33AAECA1234A1Z9", custGSTIN: "",
			place: "27-Maharashtra", wantMode: TaxIGST,
		},
		{
			// Seller GSTIN not filled in yet: the Settings state code stands in.
			name:        "seller falls back to configured state code",
			sellerState: "33", custGSTIN: "33AAAAA0000A1Z5",
			place: "", wantMode: TaxCGSTSGST,
		},
		{
			// A GSTIN zero-pads Delhi as "07"; a hand-typed state code may not.
			name:        "leading zero state codes compare equal",
			sellerState: "7", custGSTIN: "07AAAAA0000A1Z5",
			place: "", wantMode: TaxCGSTSGST,
		},
		{
			// Nothing identifiable on either side: charging CGST+SGST would
			// assert an intra-state supply nobody stated, so default to IGST.
			name:     "no state information anywhere",
			wantMode: TaxIGST,
		},
		{
			name:        "half-typed GSTIN falls through to place of supply",
			sellerGSTIN: "33AAECA1234A1Z9", custGSTIN: "3",
			place: "33-Tamil Nadu", wantMode: TaxCGSTSGST,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inv := domesticInvoice(c.sellerGSTIN, c.sellerState, c.custGSTIN, c.place)
			res := ComputeInvoice(inv)
			if res.Mode != c.wantMode {
				t.Fatalf("Mode = %v, want %v", res.Mode, c.wantMode)
			}
			if c.wantMode == TaxCGSTSGST {
				// 18% split equally: 9% + 9% on 10,000.00 = 900.00 each.
				if res.CGST != res.SGST {
					t.Errorf("CGST (%d) and SGST (%d) must be equal halves", res.CGST, res.SGST)
				}
				if got, want := res.CGST, money.Amount(90000); got != want {
					t.Errorf("CGST = %d, want %d", got, want)
				}
				if res.IGST != 0 {
					t.Errorf("IGST = %d, want 0 for an intra-state supply", res.IGST)
				}
			} else {
				if got, want := res.IGST, money.Amount(180000); got != want {
					t.Errorf("IGST = %d, want %d", got, want)
				}
				if res.CGST != 0 || res.SGST != 0 {
					t.Errorf("CGST/SGST = %d/%d, want 0/0 for an inter-state supply", res.CGST, res.SGST)
				}
			}
			// Whichever way the supply is treated, the total tax charged is
			// the same 18% — the split changes, the burden does not.
			if got, want := res.TotalTax, money.Amount(180000); got != want {
				t.Errorf("TotalTax = %d, want %d", got, want)
			}
		})
	}
}

// TestComputeInvoice_ExportIgnoresGSTINStates guards the scoping of the rule:
// only domestic invoices split, however the two GSTINs happen to line up.
func TestComputeInvoice_ExportIgnoresGSTINStates(t *testing.T) {
	for _, typ := range []model.InvoiceType{model.InvoiceExportLUT, model.InvoiceExportIGST} {
		inv := domesticInvoice("33AAECA1234A1Z9", "33", "33AAAAA0000A1Z5", "33-Tamil Nadu")
		inv.Type = typ
		res := ComputeInvoice(inv)
		if res.Mode != TaxIGST {
			t.Errorf("%s: Mode = %v, want TaxIGST", typ, res.Mode)
		}
		if res.CGST != 0 || res.SGST != 0 {
			t.Errorf("%s: CGST/SGST = %d/%d, want 0/0", typ, res.CGST, res.SGST)
		}
	}
}

func TestRateLabel(t *testing.T) {
	cases := map[int32]string{
		1800: "@18%",
		0:    "@0%",
		1250: "@12.5%",
		925:  "@9.25%",
	}
	for bps, want := range cases {
		if got := RateLabel(bps); got != want {
			t.Errorf("RateLabel(%d) = %q, want %q", bps, got, want)
		}
	}
}

func TestComputeLine_DiscountApplied(t *testing.T) {
	inv := &model.Invoice{
		Type:     model.InvoiceExportLUT,
		FXFactor: mustRate(t, "80"),
		Seller:   model.Company{StateCode: "33"},
	}
	li := model.LineItem{
		Quantity:    money.Qty(200),      // 2.00
		RateUSD:     money.Amount(10000), // 100.00
		DiscountUSD: money.Amount(5000),  // 50.00
	}
	lr := ComputeLine(inv, li)
	// gross = 2 * 100.00 = 200.00, minus 50.00 discount = 150.00 USD
	if got, want := lr.TaxableUSD, money.Amount(15000); got != want {
		t.Errorf("TaxableUSD = %d, want %d", got, want)
	}
	// 150.00 * 80 = 12,000.00 INR
	if got, want := lr.TaxableINR, money.Amount(1200000); got != want {
		t.Errorf("TaxableINR = %d, want %d", got, want)
	}
}
