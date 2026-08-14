// Command dock-invoice is the desktop invoice generator. Running it with no
// flags launches the Fyne desktop UI (Invoices / Editor / Settings /
// Masters). The --render-fixture dev flag renders a JSON fixture straight
// to PDF and exits, without opening any UI — handy for iterating on the PDF
// layout or for headless/CI use.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
	"dock-invoice/internal/pdf"
	"dock-invoice/internal/ui"
)

func main() {
	renderFixture := flag.String("render-fixture", "", "path to a fixture JSON file to render to PDF and exit (dev-only, no UI)")
	out := flag.String("out", "", "output PDF path (defaults to <Number>.pdf next to the fixture)")
	flag.Parse()

	if *renderFixture != "" {
		if err := renderFixtureFile(*renderFixture, *out); err != nil {
			fmt.Fprintf(os.Stderr, "dock-invoice: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := runGUI(); err != nil {
		fmt.Fprintf(os.Stderr, "dock-invoice: %v\n", err)
		os.Exit(1)
	}
}

// runGUI opens the workspace (the organization registry in
// os.UserConfigDir()/InvoiceGenerator plus the active organization's database,
// or the single database named by INVOICER_DB_PATH) and launches the main
// window.
//
// Closing goes through the App rather than the database opened here: switching
// organizations replaces that handle, so the database open at exit is not
// necessarily this one.
func runGUI() error {
	ws, err := ui.OpenWorkspace()
	if err != nil {
		return err
	}

	app, err := ui.NewWorkspaceApp(ws)
	if err != nil {
		ws.Close()
		return fmt.Errorf("build app: %w", err)
	}
	defer app.Close()

	app.Run()
	return nil
}

func renderFixtureFile(path, outPath string) error {
	inv, err := loadFixtureInvoice(path)
	if err != nil {
		return fmt.Errorf("load fixture: %w", err)
	}

	data, err := pdf.Render(inv)
	if err != nil {
		return fmt.Errorf("render pdf: %w", err)
	}

	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(path), sanitizeFilename(inv.Number)+".pdf")
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}
	fmt.Printf("wrote %s\n", outPath)
	return nil
}

func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "invoice"
	}
	return string(out)
}

// ---- Fixture JSON schema -------------------------------------------------
//
// A human-friendly on-disk format (decimal-string money amounts, plain
// dates) that gets parsed into the internal model.Invoice via the money
// package's exact-decimal parsers. This is intentionally separate from
// model.Invoice's own JSON encoding (used by internal/store for the actual
// database snapshot), which stores minor-unit integers.

type fixtureCompany struct {
	Name          string   `json:"name"`
	AddressLines  []string `json:"addressLines"`
	City          string   `json:"city"`
	StateCode     string   `json:"stateCode"`
	StateName     string   `json:"stateName"`
	Pincode       string   `json:"pincode"`
	Phone         string   `json:"phone"`
	Email         string   `json:"email"`
	GSTIN         string   `json:"gstin"`
	PAN           string   `json:"pan"`
	LogoPath      string   `json:"logoPath"`
	SignaturePath string   `json:"signaturePath"`
}

type fixtureBank struct {
	AccountNumber string `json:"accountNumber"`
	IFSC          string `json:"ifsc"`
	BankName      string `json:"bankName"`
	BranchName    string `json:"branchName"`
	SwiftCode     string `json:"swiftCode"`
}

type fixtureCustomer struct {
	Name            string   `json:"name"`
	GSTIN           string   `json:"gstin"`
	BillingAddress  []string `json:"billingAddress"`
	ShippingAddress []string `json:"shippingAddress"`
}

type fixtureLineItem struct {
	Description string `json:"description"`
	HSNSAC      string `json:"hsnSac"`
	Quantity    string `json:"quantity"`
	Unit        string `json:"unit"`
	RateUSD     string `json:"rateUSD"`
	DiscountUSD string `json:"discountUSD"`
	TaxRatePct  string `json:"taxRatePct"` // e.g. "18" or "18.5", in percent
	CessRatePct string `json:"cessRatePct"`
}

type fixtureInvoice struct {
	Type             string            `json:"type"`        // export_lut | export_igst | domestic
	LayoutStyle      string            `json:"layoutStyle"` // modern | classic; empty renders Modern
	Number           string            `json:"number"`
	ReferenceNo      string            `json:"referenceNo"`
	Date             string            `json:"date"`    // YYYY-MM-DD
	DueDate          string            `json:"dueDate"` // YYYY-MM-DD
	CopyType         string            `json:"copyType"`
	PlaceOfSupply    string            `json:"placeOfSupply"`
	CountryOfSupply  string            `json:"countryOfSupply"`
	Currency         string            `json:"currency"`
	FXFactor         string            `json:"fxFactor"`
	ShippingBillNo   string            `json:"shippingBillNo"`
	ShippingBillDate string            `json:"shippingBillDate"` // YYYY-MM-DD, optional
	ShippingPortCode string            `json:"shippingPortCode"`
	Customer         fixtureCustomer   `json:"customer"`
	Seller           fixtureCompany    `json:"seller"`
	Bank             fixtureBank       `json:"bank"`
	Items            []fixtureLineItem `json:"items"`
	Notes            string            `json:"notes"`
}

func loadFixtureInvoice(path string) (*model.Invoice, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fx fixtureInvoice
	if err := json.Unmarshal(raw, &fx); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	date, err := parseDate(fx.Date)
	if err != nil {
		return nil, fmt.Errorf("date: %w", err)
	}
	dueDate, err := parseDate(fx.DueDate)
	if err != nil {
		return nil, fmt.Errorf("dueDate: %w", err)
	}
	fxFactor, err := money.ParseRate(fx.FXFactor)
	if err != nil {
		return nil, fmt.Errorf("fxFactor: %w", err)
	}

	var shipDate *time.Time
	if fx.ShippingBillDate != "" {
		t, err := parseDate(fx.ShippingBillDate)
		if err != nil {
			return nil, fmt.Errorf("shippingBillDate: %w", err)
		}
		shipDate = &t
	}

	items := make([]model.LineItem, 0, len(fx.Items))
	for i, it := range fx.Items {
		qty, err := money.ParseQty(it.Quantity)
		if err != nil {
			return nil, fmt.Errorf("items[%d].quantity: %w", i, err)
		}
		rate, err := parseUSDAmount(it.RateUSD)
		if err != nil {
			return nil, fmt.Errorf("items[%d].rateUSD: %w", i, err)
		}
		discount, err := parseUSDAmount(it.DiscountUSD)
		if err != nil {
			return nil, fmt.Errorf("items[%d].discountUSD: %w", i, err)
		}
		taxBps, err := parsePercentToBps(it.TaxRatePct)
		if err != nil {
			return nil, fmt.Errorf("items[%d].taxRatePct: %w", i, err)
		}
		cessBps, err := parsePercentToBps(it.CessRatePct)
		if err != nil {
			return nil, fmt.Errorf("items[%d].cessRatePct: %w", i, err)
		}
		items = append(items, model.LineItem{
			Description: it.Description,
			HSNSAC:      it.HSNSAC,
			Quantity:    qty,
			Unit:        it.Unit,
			RateUSD:     rate,
			DiscountUSD: discount,
			TaxRatePct:  taxBps,
			CessRatePct: cessBps,
		})
	}

	inv := &model.Invoice{
		Type:             model.InvoiceType(fx.Type),
		LayoutStyle:      model.NormalizeLayoutStyle(model.LayoutStyle(fx.LayoutStyle)),
		Number:           fx.Number,
		ReferenceNo:      fx.ReferenceNo,
		Date:             date,
		DueDate:          dueDate,
		CopyType:         fx.CopyType,
		PlaceOfSupply:    fx.PlaceOfSupply,
		CountryOfSupply:  fx.CountryOfSupply,
		Currency:         fx.Currency,
		FXFactor:         fxFactor,
		ShippingBillNo:   fx.ShippingBillNo,
		ShippingBillDate: shipDate,
		ShippingPortCode: fx.ShippingPortCode,
		Items:            items,
		Notes:            fx.Notes,
		Customer: model.Customer{
			Name:            fx.Customer.Name,
			GSTIN:           fx.Customer.GSTIN,
			BillingAddress:  fx.Customer.BillingAddress,
			ShippingAddress: fx.Customer.ShippingAddress,
		},
		Seller: model.Company{
			Name:          fx.Seller.Name,
			AddressLines:  fx.Seller.AddressLines,
			City:          fx.Seller.City,
			StateCode:     fx.Seller.StateCode,
			StateName:     fx.Seller.StateName,
			Pincode:       fx.Seller.Pincode,
			Phone:         fx.Seller.Phone,
			Email:         fx.Seller.Email,
			GSTIN:         fx.Seller.GSTIN,
			PAN:           fx.Seller.PAN,
			LogoPath:      fx.Seller.LogoPath,
			SignaturePath: fx.Seller.SignaturePath,
		},
		Bank: model.Bank{
			AccountNumber: fx.Bank.AccountNumber,
			IFSC:          fx.Bank.IFSC,
			BankName:      fx.Bank.BankName,
			BranchName:    fx.Bank.BranchName,
			SwiftCode:     fx.Bank.SwiftCode,
		},
	}
	return inv, nil
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}

// parseUSDAmount parses a decimal dollar string ("4000.00") into cents.
func parseUSDAmount(s string) (money.Amount, error) {
	if s == "" {
		s = "0"
	}
	v, err := money.ParseRateDigits(s, 2)
	if err != nil {
		return 0, err
	}
	return money.Amount(v), nil
}

// parsePercentToBps parses a percent string ("18" or "18.5") into basis
// points scaled by 100 (1800, 1850).
func parsePercentToBps(s string) (int32, error) {
	if s == "" {
		s = "0"
	}
	v, err := money.ParseRateDigits(s, 2)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}
