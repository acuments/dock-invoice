// Package words converts monetary amounts to their English words
// representation: Indian numbering (lakh/crore) for INR, and standard
// dollars/cents for USD.
package words

import (
	"fmt"
	"strings"

	"dock-invoice/internal/money"
)

var ones = []string{
	"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine",
	"Ten", "Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen",
	"Seventeen", "Eighteen", "Nineteen",
}

var tens = []string{
	"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety",
}

// twoDigits converts 0-99 to words.
func twoDigits(n int64) string {
	if n < 20 {
		return ones[n]
	}
	t := tens[n/10]
	o := n % 10
	if o == 0 {
		return t
	}
	return t + " " + ones[o]
}

// threeDigits converts 0-999 to words.
func threeDigits(n int64) string {
	if n < 100 {
		return twoDigits(n)
	}
	h := n / 100
	rem := n % 100
	s := ones[h] + " Hundred"
	if rem > 0 {
		s += " " + twoDigits(rem)
	}
	return s
}

// IndianInteger converts a non-negative integer to words using the Indian
// numbering system (crore, lakh, thousand, hundred).
func IndianInteger(n int64) string {
	if n == 0 {
		return "Zero"
	}
	if n < 0 {
		return "Minus " + IndianInteger(-n)
	}

	crore := n / 10000000
	n %= 10000000
	lakh := n / 100000
	n %= 100000
	thousand := n / 1000
	n %= 1000
	hundred := n

	var parts []string
	if crore > 0 {
		parts = append(parts, threeDigits(crore)+" Crore")
	}
	if lakh > 0 {
		parts = append(parts, twoDigits(lakh)+" Lakh")
	}
	if thousand > 0 {
		parts = append(parts, twoDigits(thousand)+" Thousand")
	}
	if hundred > 0 {
		parts = append(parts, threeDigits(hundred))
	}
	return strings.Join(parts, " ")
}

// WesternInteger converts a non-negative integer to words using the Western
// (thousand/million/billion) numbering system, used for USD amounts.
func WesternInteger(n int64) string {
	if n == 0 {
		return "Zero"
	}
	if n < 0 {
		return "Minus " + WesternInteger(-n)
	}

	billion := n / 1_000_000_000
	n %= 1_000_000_000
	million := n / 1_000_000
	n %= 1_000_000
	thousand := n / 1_000
	n %= 1_000
	hundred := n

	var parts []string
	if billion > 0 {
		parts = append(parts, threeDigits(billion)+" Billion")
	}
	if million > 0 {
		parts = append(parts, threeDigits(million)+" Million")
	}
	if thousand > 0 {
		parts = append(parts, threeDigits(thousand)+" Thousand")
	}
	if hundred > 0 {
		parts = append(parts, threeDigits(hundred))
	}
	return strings.Join(parts, " ")
}

// INR renders a paise Amount as "<Indian words> Rupees Only" (paise are
// dropped, matching the sample document's whole-rupee wording). If the
// amount has non-zero paise, they are appended as "and N Paise".
func INR(a money.Amount) string {
	v := int64(a)
	neg := v < 0
	if neg {
		v = -v
	}
	rupees := v / 100
	paise := v % 100
	s := IndianInteger(rupees) + " Rupees"
	if paise > 0 {
		s += " and " + twoDigits(paise) + " Paise"
	}
	s += " Only"
	if neg {
		s = "Minus " + s
	}
	return s
}

// USD renders a cents Amount as "<Western words> USD And <Western words>
// Cent Only", matching the sample document's wording, e.g. "Four Thousand
// USD And Zero Cent Only".
func USD(a money.Amount) string {
	v := int64(a)
	neg := v < 0
	if neg {
		v = -v
	}
	dollars := v / 100
	cents := v % 100
	s := fmt.Sprintf("%s USD And %s Cent Only", WesternInteger(dollars), WesternInteger(cents))
	if neg {
		s = "Minus " + s
	}
	return s
}
