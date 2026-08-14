package calc

import "dock-invoice/internal/money"

// HSNSummaryRow is one aggregated row of an HSN/SAC-wise tax summary.
// Rows group line items that share the same HSN/SAC code and tax rate
// (basis points), which is the unit GST returns expect for HSN reporting.
type HSNSummaryRow struct {
	HSNSAC     string
	TaxRatePct int32 // full line rate in basis points (e.g. 1800 == 18%)
	TaxableINR money.Amount
	CGST       money.Amount
	SGST       money.Amount
	IGST       money.Amount
	CESS       money.Amount
	TotalTax   money.Amount
}

// HSNSummary aggregates line results by HSN/SAC + tax rate, preserving the
// order of first appearance. Used for the mandatory HSN-wise table on
// domestic invoices.
func HSNSummary(result InvoiceResult) []HSNSummaryRow {
	if len(result.Lines) == 0 {
		return nil
	}

	type key struct {
		hsn  string
		rate int32
	}
	index := make(map[key]int, len(result.Lines))
	rows := make([]HSNSummaryRow, 0, len(result.Lines))

	for _, lr := range result.Lines {
		k := key{hsn: lr.Line.HSNSAC, rate: lr.Line.TaxRatePct}
		if i, ok := index[k]; ok {
			rows[i].TaxableINR += lr.TaxableINR
			rows[i].CGST += lr.CGST
			rows[i].SGST += lr.SGST
			rows[i].IGST += lr.IGST
			rows[i].CESS += lr.CESS
			rows[i].TotalTax += lr.TotalTaxINR
			continue
		}
		index[k] = len(rows)
		rows = append(rows, HSNSummaryRow{
			HSNSAC:     lr.Line.HSNSAC,
			TaxRatePct: lr.Line.TaxRatePct,
			TaxableINR: lr.TaxableINR,
			CGST:       lr.CGST,
			SGST:       lr.SGST,
			IGST:       lr.IGST,
			CESS:       lr.CESS,
			TotalTax:   lr.TotalTaxINR,
		})
	}
	return rows
}

// PercentLabel formats a tax rate in basis points as a plain percentage
// string for HSN summary cells, e.g. 900 -> "9%", 1800 -> "18%",
// 1250 -> "12.5%". Unlike RateLabel it omits the leading "@".
func PercentLabel(bps int32) string {
	s := RateLabel(bps)
	if len(s) > 0 && s[0] == '@' {
		return s[1:]
	}
	return s
}
