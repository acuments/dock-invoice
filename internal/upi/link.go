package upi

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"dock-invoice/internal/money"
)

// PaymentURI builds a deep link of the form
// upi://pay?pa=<VPA>&cu=INR&am=<amount>&tn=<note>.
//
// Payee name (pn) is intentionally omitted: UPI apps show the verified
// bank name for the VPA, which avoids "payee name mismatch" warnings when
// the business name in settings differs from the registered account name.
func PaymentURI(vpa string, amount money.Amount, note string) string {
	vpa = strings.TrimSpace(vpa)
	if vpa == "" {
		return ""
	}

	// Build manually so @ in the VPA stays unescaped (shorter, less dense QR).
	var parts []string
	parts = append(parts, "pa="+vpa, "cu=INR")
	if amount > 0 {
		parts = append(parts, "am="+formatAmount(amount))
	}
	if n := strings.TrimSpace(note); n != "" {
		if len(n) > 40 {
			n = n[:40]
		}
		parts = append(parts, "tn="+url.QueryEscape(n))
	}
	return "upi://pay?" + strings.Join(parts, "&")
}

// formatAmount renders a paise Amount as a decimal rupee string, e.g.
// 15930000 paise -> "159300.00". UPI expects up to two fractional digits.
func formatAmount(a money.Amount) string {
	neg := a < 0
	v := int64(a)
	if neg {
		v = -v
	}
	major := v / 100
	minor := v % 100
	s := strconv.FormatInt(major, 10) + "." + fmt.Sprintf("%02d", minor)
	if neg {
		s = "-" + s
	}
	return s
}
