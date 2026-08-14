// Package model defines the core domain types shared across persistence,
// tax calculation and PDF rendering. It contains no I/O and no UI concerns.
package model

import (
	"time"

	"dock-invoice/internal/money"
)

// InvoiceType identifies which of the three supported GST treatments an
// invoice uses.
type InvoiceType string

const (
	// InvoiceExportLUT is an export invoice made under a Letter of
	// Undertaking / bond: tax amount is always 0.00, but the configured
	// tax rate label (e.g. "@18%") still prints.
	InvoiceExportLUT InvoiceType = "export_lut"
	// InvoiceExportIGST is an export invoice with IGST charged and paid;
	// shipping bill number, date and port become required fields.
	InvoiceExportIGST InvoiceType = "export_igst"
	// InvoiceDomestic is a domestic (India-to-India) invoice, INR-only,
	// split into CGST+SGST (intra-state) or IGST (inter-state) based on
	// place of supply vs seller state.
	InvoiceDomestic InvoiceType = "domestic"
)

// LayoutStyle selects which PDF layout an invoice prints in.
type LayoutStyle string

const (
	// LayoutModern is the default accent-coloured layout.
	LayoutModern LayoutStyle = "modern"
	// LayoutClassic is the monochrome, fully-ruled GST tax-invoice
	// format (Tally-style) that prints legibly on a B/W printer.
	LayoutClassic LayoutStyle = "classic"
)

// NormalizeLayoutStyle maps empty/unknown values to LayoutModern so
// invoices saved before this field existed keep printing exactly as
// they always did.
func NormalizeLayoutStyle(s LayoutStyle) LayoutStyle {
	if s == LayoutClassic {
		return LayoutClassic
	}
	return LayoutModern
}

// EffectiveLayoutStyle returns the PDF layout to render. An explicit
// classic/modern choice stamped on the invoice wins; when the invoice has
// none (""), settingsDefault applies so a Settings change reaches invoices
// that never had their Layout field touched in the editor.
func EffectiveLayoutStyle(invoice, settingsDefault LayoutStyle) LayoutStyle {
	if invoice == LayoutClassic || invoice == LayoutModern {
		return invoice
	}
	return NormalizeLayoutStyle(settingsDefault)
}

// LayoutStyleLabel returns the human label used in the UI:
// LayoutModern -> "Modern (colour)", LayoutClassic -> "Classic (black & white)".
// Unknown values normalize to the modern label.
func LayoutStyleLabel(s LayoutStyle) string {
	if NormalizeLayoutStyle(s) == LayoutClassic {
		return "Classic (black & white)"
	}
	return "Modern (colour)"
}

// LayoutStyleFromLabel is the inverse of LayoutStyleLabel; ok is false
// for an unrecognised label.
func LayoutStyleFromLabel(label string) (LayoutStyle, bool) {
	switch label {
	case "Modern (colour)":
		return LayoutModern, true
	case "Classic (black & white)":
		return LayoutClassic, true
	default:
		return "", false
	}
}

// Company is the seller/sender profile printed on every invoice.
type Company struct {
	Name          string
	AddressLines  []string
	City          string
	StateCode     string // e.g. "33"
	StateName     string // e.g. "Tamil Nadu"
	Pincode       string
	Phone         string
	Email         string
	GSTIN         string
	PAN           string
	LogoPath      string
	SignaturePath string
}

// StateLabel renders "33-Tamil Nadu" as printed on the invoice. When only
// the code is configured, the official GST state name is looked up.
func (c Company) StateLabel() string {
	return GSTStateLabel(c.StateCode, c.StateName)
}

// Bank holds the seller's bank details shown in the "Bank Details" box.
type Bank struct {
	AccountNumber string
	IFSC          string
	BankName      string
	BranchName    string
	SwiftCode     string
	// UPIID is the seller's UPI Virtual Payment Address (VPA), e.g.
	// merchant@hdfc. When set, invoice PDFs include a scannable UPI QR
	// code with the invoice total pre-filled.
	UPIID string
}

// Defaults are prefilled values used when creating a new invoice.
type Defaults struct {
	HSNSAC          string
	Unit            string
	TaxRatePct      int32 // basis points, e.g. 1800 == 18.00%
	PaymentTermDays int
	Currency        string
	CopyType        string
}

// Settings is the single application configuration record.
type Settings struct {
	Company        Company
	Bank           Bank
	LogoPath       string
	SignaturePath  string
	OutputDir      string
	Defaults       Defaults
	Declarations   map[InvoiceType]string
	NumberPatterns map[InvoiceType]string
	LastFXFactor string

	// IncludeHSNSACSummary, when true, prints the HSN/SAC tax summary
	// table on domestic invoice PDFs. Off by default so existing layout
	// stays unchanged until the user opts in.
	IncludeHSNSACSummary bool

	// LayoutStyle is the default PDF layout applied to newly created
	// invoices. Empty (the zero value for rows saved before this field
	// existed) normalizes to LayoutModern via NormalizeLayoutStyle.
	LayoutStyle LayoutStyle
}

// Customer is a lookup/master record. Invoices snapshot these fields rather
// than referencing the master live, so a later edit here must never change
// an already-saved invoice.
type Customer struct {
	ID              int64
	Name            string
	GSTIN           string
	BillingAddress  []string
	ShippingAddress []string
}

// Currency codes a saved item's default rate (and, where noted, an
// invoice) may be denominated in. Only these two are supported today —
// export invoices price in USD, domestic invoices price in INR.
const (
	CurrencyUSD = "USD"
	CurrencyINR = "INR"
)

// Item is a saved line-item template (description + HSN/SAC + default
// rate/unit) used to speed up data entry. Not referenced live by invoices.
//
// DefaultRate is denominated in Currency, not always USD: large catalogues
// mix domestic (INR-priced) and export (USD-priced) items, so the master
// must be able to say which one each row means. Currency is normalized to
// CurrencyUSD when empty (the historical assumption, before this field
// existed, was that every saved rate was USD) — see store.migrateItems.
type Item struct {
	ID          int64
	Description string
	HSNSAC      string
	Unit        string
	DefaultRate money.Amount
	Currency    string
}

// LineItem is one row of an invoice's item table. All monetary fields are
// entered in USD (per the plan's "amounts entered in USD" rule) even for
// domestic invoices, where the derived INR values are what gets printed and
// the USD figures are simply not shown.
type LineItem struct {
	Description string
	HSNSAC      string
	Quantity    money.Qty
	Unit        string
	RateUSD     money.Amount
	DiscountUSD money.Amount
	TaxRatePct  int32 // basis points, e.g. 1800 == 18.00%
	CessRatePct int32
}

// Invoice is a full invoice record. Customer is a snapshot, not a foreign
// key: once saved, later edits to the customer master must not alter it.
type Invoice struct {
	ID               int64
	Type             InvoiceType
	Number           string
	ReferenceNo      string
	Date             time.Time
	DueDate          time.Time
	Customer         Customer
	PlaceOfSupply    string
	CountryOfSupply  string
	Currency         string // "USD" for export types, "INR" for domestic
	FXFactor         money.Rate
	ShippingBillNo   string
	ShippingPortCode string
	ShippingBillDate *time.Time
	Items            []LineItem
	CopyType         string
	Notes            string

	// Seller snapshot fields, captured at save time so the printed
	// invoice never changes even if Settings.Company/Bank change later.
	Seller Company
	Bank   Bank

	// IncludeHSNSACSummary is stamped from Settings at render/save time.
	// When true, domestic PDFs include the HSN/SAC tax summary table.
	IncludeHSNSACSummary bool

	// LayoutStyle is stamped from Settings at save time so a later change
	// to the default layout never alters an already-saved invoice's PDF.
	// It is persisted as part of the Invoice JSON blob, so rows saved
	// before this field existed simply unmarshal it to "" — NormalizeLayoutStyle
	// treats that the same as LayoutModern, which is what those invoices
	// always rendered as.
	LayoutStyle LayoutStyle
}
