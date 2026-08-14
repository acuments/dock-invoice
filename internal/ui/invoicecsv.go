package ui

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"dock-invoice/internal/calc"
	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

var invoiceCSVHeader = []string{
	"Date",
	"Number",
	"Reference No",
	"Type",
	"Customer",
	"Customer GSTIN",
	"Place of Supply",
	"Currency",
	"FX Factor",
	"Taxable (INR)",
	"CGST (INR)",
	"SGST (INR)",
	"IGST (INR)",
	"Cess (INR)",
	"Round Off (INR)",
	"Total (INR)",
	"Taxable (USD)",
	"Total (USD)",
}

func csvAmount(a money.Amount) string {
	neg := a < 0
	v := int64(a)
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d.%02d", v/100, v%100)
	if neg {
		s = "-" + s
	}
	return s
}

func invoiceCSVRow(inv model.Invoice) []string {
	result := calc.ComputeInvoice(&inv)

	taxableUSD, totalUSD := "", ""
	if inv.Currency == model.CurrencyUSD {
		taxableUSD = csvAmount(result.TaxableUSD)
		totalUSD = csvAmount(result.TotalUSD)
	}

	fx := ""
	if inv.FXFactor != 0 {
		fx = inv.FXFactor.String()
	}

	return []string{
		inv.Date.Format("2006-01-02"),
		inv.Number,
		inv.ReferenceNo,
		InvoiceTypeLabel(inv.Type),
		inv.Customer.Name,
		inv.Customer.GSTIN,
		inv.PlaceOfSupply,
		inv.Currency,
		fx,
		csvAmount(result.TaxableINR),
		csvAmount(result.CGST),
		csvAmount(result.SGST),
		csvAmount(result.IGST),
		csvAmount(result.CESS),
		csvAmount(result.RoundOff),
		csvAmount(result.GrandTotal),
		taxableUSD,
		totalUSD,
	}
}

func writeInvoiceCSV(w io.Writer, invs []model.Invoice) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(invoiceCSVHeader); err != nil {
		return err
	}
	for _, inv := range invs {
		if err := cw.Write(invoiceCSVRow(inv)); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func invoiceCSVFilename(f invoiceFilter, now time.Time) string {
	parts := []string{"invoices"}
	if f.fy != "" {
		parts = append(parts, strings.ToLower(strings.ReplaceAll(fyLabel(f.fy), " ", "")))
	}
	if f.month != 0 {
		parts = append(parts, strings.ToLower(f.month.String()))
	}
	if strings.TrimSpace(f.query) != "" {
		parts = append(parts, "search")
	}
	if len(parts) == 1 {
		parts = append(parts, now.Format("2006-01-02"))
	}
	return strings.Join(parts, "-") + ".csv"
}

func invoiceExportLabel(n int, path string) string {
	noun := "invoices"
	if n == 1 {
		noun = "invoice"
	}
	return "Exported " + strconv.Itoa(n) + " " + noun + " to " + path
}
