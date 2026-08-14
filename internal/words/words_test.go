package words

import (
	"testing"

	"dock-invoice/internal/money"
)

func TestINR_Golden(t *testing.T) {
	got := INR(money.Amount(33280000)) // 3,32,800.00
	want := "Three Lakh Thirty Two Thousand Eight Hundred Rupees Only"
	if got != want {
		t.Errorf("INR = %q, want %q", got, want)
	}
}

func TestUSD_Golden(t *testing.T) {
	got := USD(money.Amount(400000)) // 4,000.00
	want := "Four Thousand USD And Zero Cent Only"
	if got != want {
		t.Errorf("USD = %q, want %q", got, want)
	}
}

func TestINR_CroreScale(t *testing.T) {
	// 1,23,45,678.90 -> One Crore Twenty Three Lakh Forty Five Thousand
	// Six Hundred Seventy Eight Rupees and 90 Paise Only
	got := INR(money.Amount(1234567890))
	want := "One Crore Twenty Three Lakh Forty Five Thousand Six Hundred Seventy Eight Rupees and Ninety Paise Only"
	if got != want {
		t.Errorf("INR crore = %q, want %q", got, want)
	}
}

func TestINR_PaiseCase(t *testing.T) {
	got := INR(money.Amount(150)) // 1.50
	want := "One Rupees and Fifty Paise Only"
	if got != want {
		t.Errorf("INR paise = %q, want %q", got, want)
	}
}

func TestINR_Zero(t *testing.T) {
	got := INR(money.Amount(0))
	want := "Zero Rupees Only"
	if got != want {
		t.Errorf("INR zero = %q, want %q", got, want)
	}
}

func TestUSD_WithCents(t *testing.T) {
	got := USD(money.Amount(150025)) // 1,500.25
	want := "One Thousand Five Hundred USD And Twenty Five Cent Only"
	if got != want {
		t.Errorf("USD cents = %q, want %q", got, want)
	}
}

func TestIndianInteger(t *testing.T) {
	cases := map[int64]string{
		0:        "Zero",
		1:        "One",
		19:       "Nineteen",
		20:       "Twenty",
		99:       "Ninety Nine",
		100:      "One Hundred",
		1000:     "One Thousand",
		100000:   "One Lakh",
		10000000: "One Crore",
	}
	for n, want := range cases {
		if got := IndianInteger(n); got != want {
			t.Errorf("IndianInteger(%d) = %q, want %q", n, got, want)
		}
	}
}
