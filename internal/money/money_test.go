package money

import "testing"

func TestFormatINR(t *testing.T) {
	cases := []struct {
		paise Amount
		want  string
	}{
		{33280000, "3,32,800.00"},
		{100, "1.00"},
		{0, "0.00"},
		{123456789, "12,34,567.89"},
		{-33280000, "-3,32,800.00"},
		{99900, "999.00"},
	}
	for _, c := range cases {
		if got := FormatINR(c.paise); got != c.want {
			t.Errorf("FormatINR(%d) = %q, want %q", c.paise, got, c.want)
		}
	}
}

func TestFormatUSD(t *testing.T) {
	cases := []struct {
		cents Amount
		want  string
	}{
		{400000, "4,000.00"},
		{123456789, "1,234,567.89"},
		{0, "0.00"},
		{100, "1.00"},
	}
	for _, c := range cases {
		if got := FormatUSD(c.cents); got != c.want {
			t.Errorf("FormatUSD(%d) = %q, want %q", c.cents, got, c.want)
		}
	}
}

func TestParseRate(t *testing.T) {
	r, err := ParseRate("83.2")
	if err != nil {
		t.Fatal(err)
	}
	if int64(r) != 83_200_000 {
		t.Errorf("ParseRate(83.2) = %d, want 83200000", int64(r))
	}
	if r.String() != "83.2" {
		t.Errorf("Rate.String() = %q, want 83.2", r.String())
	}
}

func TestConvertUSDToINR_Golden(t *testing.T) {
	// USD 4,000.00 (400000 cents) x 83.2 = INR 3,32,800.00 (33280000 paise)
	rate, err := ParseRate("83.2")
	if err != nil {
		t.Fatal(err)
	}
	got := ConvertUSDToINR(Amount(400000), rate)
	want := Amount(33280000)
	if got != want {
		t.Errorf("ConvertUSDToINR = %d, want %d", got, want)
	}
}

func TestConvertUSDToINR_RoundHalfUp(t *testing.T) {
	// 1 cent * 0.5 factor scaled = exactly half a paise boundary case.
	rate := Rate(500_000) // 0.5
	got := ConvertUSDToINR(Amount(1), rate)
	// 1 * 500000 / 1000000 = 0.5 -> rounds to 1 (half up)
	if got != 1 {
		t.Errorf("ConvertUSDToINR half-paise round = %d, want 1", got)
	}
}

func TestConvertINRToUSD_Golden(t *testing.T) {
	// INR 3,32,800.00 (33280000 paise) / 83.2 = USD 4,000.00 (400000 cents)
	rate, err := ParseRate("83.2")
	if err != nil {
		t.Fatal(err)
	}
	got := ConvertINRToUSD(Amount(33280000), rate)
	want := Amount(400000)
	if got != want {
		t.Errorf("ConvertINRToUSD = %d, want %d", got, want)
	}
}

func TestConvertINRToUSD_RoundTripsWithConvertUSDToINR(t *testing.T) {
	rate, err := ParseRate("83.2")
	if err != nil {
		t.Fatal(err)
	}
	usd := Amount(400000)
	inr := ConvertUSDToINR(usd, rate)
	if got := ConvertINRToUSD(inr, rate); got != usd {
		t.Errorf("round trip ConvertINRToUSD(ConvertUSDToINR(x)) = %d, want %d", got, usd)
	}
}

func TestConvertINRToUSD_ZeroRate(t *testing.T) {
	if got := ConvertINRToUSD(Amount(1000), Rate(0)); got != 0 {
		t.Errorf("ConvertINRToUSD with zero rate = %d, want 0", got)
	}
}

func TestApplyPercent(t *testing.T) {
	// 18% of 33280000 paise
	got := ApplyPercent(Amount(33280000), 1800)
	want := Amount(5990400)
	if got != want {
		t.Errorf("ApplyPercent = %d, want %d", got, want)
	}
	// half rate (9%) for CGST/SGST split
	half := ApplyPercent(Amount(33280000), 900)
	if half != 2995200 {
		t.Errorf("ApplyPercent half = %d, want 2995200", half)
	}
}

func TestMultiplyQty(t *testing.T) {
	rate := Amount(33280000) // per unit rate in paise
	qty := Qty(100)          // 1.00
	got := MultiplyQty(rate, qty)
	if got != 33280000 {
		t.Errorf("MultiplyQty = %d, want 33280000", got)
	}
}

func TestParseQty(t *testing.T) {
	q, err := ParseQty("1.00")
	if err != nil {
		t.Fatal(err)
	}
	if q != 100 {
		t.Errorf("ParseQty(1.00) = %d, want 100", q)
	}
	if q.String() != "1.00" {
		t.Errorf("Qty.String() = %q, want 1.00", q.String())
	}
}

func TestRoundToRupee(t *testing.T) {
	rounded, off := RoundToRupee(Amount(33280050)) // 3,32,800.50
	if rounded != 33280100 {
		t.Errorf("RoundToRupee rounded = %d, want 33280100", rounded)
	}
	if off != 50 {
		t.Errorf("RoundToRupee off = %d, want 50", off)
	}
	rounded2, off2 := RoundToRupee(Amount(33280000))
	if rounded2 != 33280000 || off2 != 0 {
		t.Errorf("RoundToRupee exact = %d/%d, want 33280000/0", rounded2, off2)
	}
}
