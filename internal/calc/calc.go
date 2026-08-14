// Package calc computes per-line and invoice-level totals from a model.Invoice,
// including USD->INR conversion and the CGST/SGST/IGST split. All arithmetic
// is done in integer minor units via the money package; float64 is never
// used.
package calc

import (
	"strings"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

// TaxMode describes how a line's tax is split for display/columns.
type TaxMode int

const (
	// TaxIGST is a single IGST column (export invoices, or domestic
	// inter-state).
	TaxIGST TaxMode = iota
	// TaxCGSTSGST splits the rate in half between CGST and SGST
	// (domestic intra-state).
	TaxCGSTSGST
)

// LineResult holds the computed figures for a single invoice line, all in
// minor units (paise for INR fields, cents for USD fields).
type LineResult struct {
	Line model.LineItem

	TaxableUSD money.Amount // qty*rateUSD - discountUSD
	TaxableINR money.Amount // converted from TaxableUSD via the invoice FX factor

	// Tax amounts in INR paise. Under export_lut these are always zero
	// even though TaxRatePct/RateLabel still reflect the configured rate.
	IGST money.Amount
	CGST money.Amount
	SGST money.Amount
	CESS money.Amount

	TotalTaxINR money.Amount
	TotalINR    money.Amount // TaxableINR + TotalTaxINR
	TotalUSD    money.Amount // TaxableUSD (export types only; tax is not charged in USD)
}

// InvoiceResult holds invoice-level totals, all in minor units.
type InvoiceResult struct {
	Mode  TaxMode
	Lines []LineResult

	TaxableINR money.Amount
	IGST       money.Amount
	CGST       money.Amount
	SGST       money.Amount
	CESS       money.Amount
	TotalTax   money.Amount
	TotalINR   money.Amount // before round-off
	RoundOff   money.Amount // domestic only; zero for exports
	GrandTotal money.Amount // TotalINR + RoundOff

	TaxableUSD money.Amount
	TotalUSD   money.Amount
}

// ComputeLine computes the taxable value and tax split for a single line of
// the given invoice. Tax is always computed on the INR taxable value.
func ComputeLine(inv *model.Invoice, li model.LineItem) LineResult {
	res := LineResult{Line: li}

	gross := money.MultiplyQty(li.RateUSD, li.Quantity)
	res.TaxableUSD = gross - li.DiscountUSD

	res.TaxableINR = money.ConvertUSDToINR(res.TaxableUSD, inv.FXFactor)

	switch inv.Type {
	case model.InvoiceExportLUT:
		// Rate label still applies, but the actual tax charged is zero.
		res.IGST = 0
		res.CGST = 0
		res.SGST = 0
	case model.InvoiceExportIGST:
		res.IGST = money.ApplyPercent(res.TaxableINR, li.TaxRatePct)
	case model.InvoiceDomestic:
		if domesticSameState(inv) {
			half := li.TaxRatePct / 2
			res.CGST = money.ApplyPercent(res.TaxableINR, half)
			res.SGST = money.ApplyPercent(res.TaxableINR, half)
		} else {
			res.IGST = money.ApplyPercent(res.TaxableINR, li.TaxRatePct)
		}
	}
	res.CESS = money.ApplyPercent(res.TaxableINR, li.CessRatePct)

	res.TotalTaxINR = res.IGST + res.CGST + res.SGST + res.CESS
	res.TotalINR = res.TaxableINR + res.TotalTaxINR
	res.TotalUSD = res.TaxableUSD

	return res
}

// domesticSameState reports whether a domestic supply is intra-state, which is
// what selects an equal CGST+SGST split over a single IGST charge.
//
// Each party's state is read from the first two digits of its GSTIN: the GST
// state code is the leading two characters of every 15-character GSTIN, and
// both parties' registration numbers are printed on the invoice itself, so
// this is the most reliable signal available.
//
// Either side falls back when its GSTIN is absent or malformed:
//
//   - the seller falls back to the State code entered in Settings;
//   - the recipient falls back to the place-of-supply code. That fallback
//     matters: an unregistered domestic buyer has no GSTIN at all, and for
//     them place of supply is the only thing that can decide the split.
//
// Note that where a registered buyer's GSTIN state and the place of supply
// disagree (a bill-to/ship-to supply of goods, say), this treats the GSTIN as
// authoritative.
func domesticSameState(inv *model.Invoice) bool {
	seller := firstStateCode(
		stateCodeFromGSTIN(inv.Seller.GSTIN),
		inv.Seller.StateCode,
	)
	recipient := firstStateCode(
		stateCodeFromGSTIN(inv.Customer.GSTIN),
		stateCodePrefix(inv.PlaceOfSupply),
	)
	return seller != "" && seller == recipient
}

// stateCodeFromGSTIN extracts the GST state code from the first two characters
// of a GSTIN. It returns "" for anything that does not start with two digits,
// so a blank or half-typed GSTIN falls through to the caller's fallback rather
// than silently comparing as a state.
func stateCodeFromGSTIN(gstin string) string {
	g := strings.TrimSpace(gstin)
	if len(g) < 2 || !isDigit(g[0]) || !isDigit(g[1]) {
		return ""
	}
	return normalizeStateCode(g[:2])
}

// stateCodePrefix takes the leading code from a place of supply formatted as
// "33-Tamil Nadu"; a bare "33" is accepted too.
func stateCodePrefix(placeOfSupply string) string {
	for i, r := range placeOfSupply {
		if r == '-' {
			return normalizeStateCode(placeOfSupply[:i])
		}
	}
	return normalizeStateCode(placeOfSupply)
}

// firstStateCode returns the first non-empty normalised code from candidates.
func firstStateCode(candidates ...string) string {
	for _, c := range candidates {
		if code := normalizeStateCode(c); code != "" {
			return code
		}
	}
	return ""
}

// normalizeStateCode puts a state code into one comparable form. A GSTIN
// always zero-pads ("07" for Delhi) while someone filling in Settings by hand
// may well type "7"; without this the two would never compare equal and a
// Delhi-to-Delhi supply would be charged IGST.
func normalizeStateCode(s string) string {
	s = strings.TrimSpace(s)
	trimmed := strings.TrimLeft(s, "0")
	if trimmed == "" {
		// All zeros is not a real state code (they run 01-38).
		return ""
	}
	return trimmed
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// TaxModeFor determines the column layout for an invoice: CGST/SGST for
// domestic intra-state, IGST otherwise.
func TaxModeFor(inv *model.Invoice) TaxMode {
	if inv.Type == model.InvoiceDomestic && domesticSameState(inv) {
		return TaxCGSTSGST
	}
	return TaxIGST
}

// ComputeInvoice computes all line results and invoice-level totals.
func ComputeInvoice(inv *model.Invoice) InvoiceResult {
	result := InvoiceResult{Mode: TaxModeFor(inv)}

	for _, li := range inv.Items {
		lr := ComputeLine(inv, li)
		result.Lines = append(result.Lines, lr)

		result.TaxableINR += lr.TaxableINR
		result.IGST += lr.IGST
		result.CGST += lr.CGST
		result.SGST += lr.SGST
		result.CESS += lr.CESS
		result.TaxableUSD += lr.TaxableUSD
		result.TotalUSD += lr.TotalUSD
	}

	result.TotalTax = result.IGST + result.CGST + result.SGST + result.CESS
	result.TotalINR = result.TaxableINR + result.TotalTax

	if inv.Type == model.InvoiceDomestic {
		rounded, off := money.RoundToRupee(result.TotalINR)
		result.RoundOff = off
		result.GrandTotal = rounded
	} else {
		result.RoundOff = 0
		result.GrandTotal = result.TotalINR
	}

	return result
}

// RateLabel formats a line's configured tax rate as printed under the tax
// amount, e.g. "@18%". Basis points (1800) -> "18%"; fractional rates such
// as 1250 (12.5%) print as "@12.5%".
func RateLabel(bps int32) string {
	whole := bps / 100
	frac := bps % 100
	if frac == 0 {
		return formatPct(whole, -1)
	}
	if frac%10 == 0 {
		return formatPct(whole, frac/10)
	}
	return formatPctFrac2(whole, frac)
}

func formatPct(whole int32, oneDecimal int32) string {
	if oneDecimal < 0 {
		return "@" + itoa(int64(whole)) + "%"
	}
	return "@" + itoa(int64(whole)) + "." + itoa(int64(oneDecimal)) + "%"
}

func formatPctFrac2(whole, frac int32) string {
	return "@" + itoa(int64(whole)) + "." + pad2(frac) + "%"
}

func pad2(n int32) string {
	s := itoa(int64(n))
	if len(s) < 2 {
		return "0" + s
	}
	return s
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
