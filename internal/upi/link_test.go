package upi

import (
	"net/url"
	"strings"
	"testing"

	"dock-invoice/internal/money"
)

func TestPaymentURI(t *testing.T) {
	uri := PaymentURI("merchant@hdfc", money.Amount(50000), "INV-42")
	if !strings.HasPrefix(uri, "upi://pay?") {
		t.Fatalf("unexpected prefix: %q", uri)
	}
	for _, want := range []string{"pa=merchant", "cu=INR", "am=500.00", "tn=INV-42"} {
		if !strings.Contains(uri, want) {
			t.Errorf("URI %q missing %q", uri, want)
		}
	}
	if strings.Contains(uri, "pn=") {
		t.Errorf("URI should omit payee name: %q", uri)
	}
}

func TestPaymentURI_EmptyVPA(t *testing.T) {
	if got := PaymentURI("", money.Amount(100), ""); got != "" {
		t.Errorf("empty VPA => %q, want empty", got)
	}
}

func TestPaymentURI_TruncatesNote(t *testing.T) {
	long := strings.Repeat("x", 100)
	uri := PaymentURI("a@b", 0, long)
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	note, err := url.QueryUnescape(u.Query().Get("tn"))
	if err != nil {
		t.Fatal(err)
	}
	if len(note) != 40 {
		t.Errorf("note length = %d, want 40", len(note))
	}
}

func TestPaymentURI_KeepsAtUnescaped(t *testing.T) {
	uri := PaymentURI("merchant@hdfc", money.Amount(100), "")
	if strings.Contains(uri, "%40") {
		t.Errorf("VPA @ should stay unescaped for a sparser QR: %q", uri)
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		in   money.Amount
		want string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{100, "1.00"},
		{50000, "500.00"},
		{15930000, "159300.00"},
	}
	for _, tc := range tests {
		if got := formatAmount(tc.in); got != tc.want {
			t.Errorf("formatAmount(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPNG(t *testing.T) {
	png, err := PNG(PaymentURI("test@hdfc", money.Amount(100), ""), 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 8 || png[0] != 0x89 {
		t.Fatalf("expected PNG header, got %d bytes", len(png))
	}
}
