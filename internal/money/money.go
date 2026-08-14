// Package money represents monetary and quantity values as fixed-point
// integers so that invoice arithmetic is exact and reproducible.
//
// Amounts are always stored and manipulated as int64 minor units (paise for
// INR, cents for USD). float64 must never be used for stored or derived
// monetary amounts anywhere in this codebase.
package money

import (
	"fmt"
	"strconv"
	"strings"
)

// Amount is a monetary value expressed in minor units (paise or cents).
type Amount int64

// RateScale is the fixed-point scale used to represent decimal conversion
// factors (e.g. 83.2 -> 83_200_000) without floating point.
const RateScale int64 = 1_000_000

// Rate is a decimal conversion factor scaled by RateScale.
type Rate int64

// ParseRate parses a plain decimal string such as "83.2" into a Rate scaled
// by RateScale. It supports up to 6 fractional digits (extra digits are
// rounded half-up).
func ParseRate(s string) (Rate, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("money: empty rate")
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	whole := s
	frac := ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		whole = s[:i]
		frac = s[i+1:]
	}
	if whole == "" {
		whole = "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("money: invalid rate %q: %w", s, err)
	}
	// Pad or truncate (with rounding) frac to 6 digits.
	fracScaled, err := scaleFrac(frac, 6)
	if err != nil {
		return 0, fmt.Errorf("money: invalid rate %q: %w", s, err)
	}
	total := w*RateScale + fracScaled
	if neg {
		total = -total
	}
	return Rate(total), nil
}

// scaleFrac converts a fractional-digit string (digits after the decimal
// point) into an integer scaled to `digits` decimal places, rounding
// half-up if the input has more precision than `digits`.
func scaleFrac(frac string, digits int) (int64, error) {
	if frac == "" {
		return 0, nil
	}
	for _, c := range frac {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid fraction %q", frac)
		}
	}
	if len(frac) <= digits {
		frac = frac + strings.Repeat("0", digits-len(frac))
		v, err := strconv.ParseInt(frac, 10, 64)
		return v, err
	}
	kept := frac[:digits]
	roundDigit := frac[digits] - '0'
	v, err := strconv.ParseInt(kept, 10, 64)
	if err != nil {
		return 0, err
	}
	if roundDigit >= 5 {
		v++
	}
	return v, nil
}

// String renders the rate back to a plain decimal string, e.g. "83.2".
func (r Rate) String() string {
	neg := r < 0
	v := int64(r)
	if neg {
		v = -v
	}
	whole := v / RateScale
	frac := v % RateScale
	fracStr := fmt.Sprintf("%06d", frac)
	fracStr = strings.TrimRight(fracStr, "0")
	s := strconv.FormatInt(whole, 10)
	if fracStr != "" {
		s = s + "." + fracStr
	}
	if neg {
		s = "-" + s
	}
	return s
}

// roundHalfUpDiv computes round(numerator/denominator) using half-up
// rounding (ties away from zero for negative numerator, per the domain's
// "round half up" convention applied to the value's own sign).
func roundHalfUpDiv(numerator, denominator int64) int64 {
	if denominator < 0 {
		numerator, denominator = -numerator, -denominator
	}
	if numerator >= 0 {
		return (numerator + denominator/2) / denominator
	}
	return -((-numerator + denominator/2) / denominator)
}

// ConvertUSDToINR converts a USD amount (cents) to INR (paise) using the
// scaled conversion factor, rounding half-up:
//
//	taxableINR = round((taxableUSD_cents * factor_scaled) / 1e6)
func ConvertUSDToINR(usd Amount, rate Rate) Amount {
	return Amount(roundHalfUpDiv(int64(usd)*int64(rate), RateScale))
}

// ConvertINRToUSD converts an INR amount (paise) to USD (cents) using the
// scaled conversion factor, rounding half-up. This is the inverse of
// ConvertUSDToINR:
//
//	usdCents = round((inrPaise * 1e6) / factorScaled)
//
// Used when a saved item's default rate is priced in one currency but is
// being pasted onto a line item priced in the other (e.g. an INR-priced
// item added to a USD export invoice). Returns 0 for a zero/unset rate.
func ConvertINRToUSD(inr Amount, rate Rate) Amount {
	if rate == 0 {
		return 0
	}
	return Amount(roundHalfUpDiv(int64(inr)*RateScale, int64(rate)))
}

// Qty is a quantity expressed in hundredths (2 decimal places), e.g. 1.00
// unit is stored as 100.
type Qty int64

// ParseQty parses a decimal string such as "1.00" or "2.5" into a Qty.
func ParseQty(s string) (Qty, error) {
	r, err := ParseRateDigits(s, 2)
	return Qty(r), err
}

// ParseRateDigits parses a decimal string into an integer scaled to the
// given number of decimal digits, rounding half-up on excess precision.
func ParseRateDigits(s string, digits int) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("money: empty value")
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	whole := s
	frac := ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		whole = s[:i]
		frac = s[i+1:]
	}
	if whole == "" {
		whole = "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("money: invalid value %q: %w", s, err)
	}
	scale := int64(1)
	for i := 0; i < digits; i++ {
		scale *= 10
	}
	fracScaled, err := scaleFrac(frac, digits)
	if err != nil {
		return 0, fmt.Errorf("money: invalid value %q: %w", s, err)
	}
	total := w*scale + fracScaled
	if neg {
		total = -total
	}
	return total, nil
}

// String renders the quantity to 2 decimal places, e.g. "1.00".
func (q Qty) String() string {
	neg := q < 0
	v := int64(q)
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d.%02d", v/100, v%100)
	if neg {
		s = "-" + s
	}
	return s
}

// MultiplyQty multiplies a per-unit Amount by a Qty, rounding half-up to the
// nearest minor unit.
func MultiplyQty(rate Amount, qty Qty) Amount {
	return Amount(roundHalfUpDiv(int64(rate)*int64(qty), 100))
}

// ApplyPercent computes amount * bps / 10000 (bps = basis points scaled so
// that 1800 == 18.00%), rounding half-up. Used for tax and cess rates.
func ApplyPercent(amount Amount, bps int32) Amount {
	return Amount(roundHalfUpDiv(int64(amount)*int64(bps), 10000))
}

// FormatINR formats a paise Amount using Indian digit grouping, e.g.
// 33280000 paise -> "3,32,800.00".
func FormatINR(a Amount) string {
	return format(a, true)
}

// FormatUSD formats a cents Amount using Western digit grouping, e.g.
// 400000 cents -> "4,000.00".
func FormatUSD(a Amount) string {
	return format(a, false)
}

func format(minor Amount, indian bool) string {
	neg := minor < 0
	v := int64(minor)
	if neg {
		v = -v
	}
	major := v / 100
	frac := v % 100
	intStr := strconv.FormatInt(major, 10)
	var grouped string
	if indian {
		grouped = groupIndian(intStr)
	} else {
		grouped = groupWestern(intStr)
	}
	s := fmt.Sprintf("%s.%02d", grouped, frac)
	if neg {
		s = "-" + s
	}
	return s
}

// groupIndian groups digits as 3 then pairs of 2 from the right, e.g.
// "332800" -> "3,32,800".
func groupIndian(s string) string {
	if len(s) <= 3 {
		return s
	}
	last3 := s[len(s)-3:]
	rest := s[:len(s)-3]
	var parts []string
	for len(rest) > 2 {
		parts = append([]string{rest[len(rest)-2:]}, parts...)
		rest = rest[:len(rest)-2]
	}
	if len(rest) > 0 {
		parts = append([]string{rest}, parts...)
	}
	parts = append(parts, last3)
	return strings.Join(parts, ",")
}

// groupWestern groups digits in threes from the right, e.g. "4000" ->
// "4,000".
func groupWestern(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}
	var parts []string
	for n > 3 {
		parts = append([]string{s[n-3 : n]}, parts...)
		n -= 3
	}
	parts = append([]string{s[:n]}, parts...)
	return strings.Join(parts, ",")
}

// RoundToRupee rounds a paise Amount to the nearest whole rupee (100 paise),
// returning the rounded amount and the round-off delta (rounded - original),
// both in paise. Used for the domestic RoundOff summary line.
func RoundToRupee(a Amount) (rounded Amount, roundOff Amount) {
	r := Amount(roundHalfUpDiv(int64(a), 100) * 100)
	return r, r - a
}
