package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"dock-invoice/internal/model"
	"dock-invoice/internal/store"
	"dock-invoice/internal/testfixture"
)

// visualOutDir is where screenshots land (override with INVOICER_VISUAL_DIR).
func visualOutDir(t *testing.T) string {
	t.Helper()
	outDir := os.Getenv("INVOICER_VISUAL_DIR")
	if outDir == "" {
		outDir = filepath.Join("..", "..", "testdata", "visual")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir visual dir: %v", err)
	}
	return outDir
}

// shoot renders content in a headless window at the given size and writes a
// PNG. The application theme is applied first — capturing under Fyne's stock
// test theme (as this file used to) produced screenshots that looked nothing
// like the shipping app, which made them useless for judging layout or colour.
func shoot(t *testing.T, name string, size fyne.Size, content fyne.CanvasObject) {
	t.Helper()
	win := test.NewTempWindow(t, content)
	win.Resize(size)
	win.Show()
	// Headless render needs a beat to lay out before the capture.
	time.Sleep(250 * time.Millisecond)
	writePNG(t, filepath.Join(visualOutDir(t), name+".png"), win.Canvas().Capture())
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create screenshot: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("write screenshot: %v", err)
	}
	t.Logf("wrote %s", path)
}

func visualSettings() model.Settings {
	return testfixture.Settings()
}

func visualEditorState(settings model.Settings) *EditorState {
	state := NewEditorState(settings)
	state.Number = "AEX2425-1"
	state.Customer.Name = "Northwind Trading Co."
	state.Customer.GSTIN = "33AAAAA0000A1Z5"
	state.Customer.BillingAddress = []string{"1209 Orange Street", "Wilmington, DE 19801", "United States"}
	state.Customer.ShippingAddress = []string{"1209 Orange Street", "Wilmington, DE 19801", "United States"}
	state.PlaceOfSupply = "97-Other Territory"
	state.CountryOfSupply = "UNITED STATES OF AMERICA"
	state.Items[0].Description = "Software development services — May 2026"
	state.Items[0].RateUSD = "4000.00"
	state.AddLine(settings.Defaults.HSNSAC, settings.Defaults.Unit, settings.Defaults.TaxRatePct)
	state.Items[1].Description = "Cloud infrastructure management"
	state.Items[1].RateUSD = "850.00"
	return state
}

func visualMasters() EditorMasters {
	return EditorMasters{
		Customers: []model.Customer{
			{ID: 1, Name: "Northwind Trading Co.", GSTIN: "33AAAAA0000A1Z5"},
			{ID: 2, Name: "Contoso Retail Pvt Ltd", GSTIN: "29BBBBB1111B2Z6"},
		},
		Items: []model.Item{
			{ID: 1, Description: "Software development services", HSNSAC: "998314", Unit: "UNT", DefaultRate: 400000, Currency: model.CurrencyUSD},
			{ID: 2, Description: "Cloud infrastructure management", HSNSAC: "998315", Unit: "UNT", DefaultRate: 85000, Currency: model.CurrencyUSD},
		},
	}
}

// seedVisualInvoices puts a handful of realistic invoices in db so the
// Invoices table screenshot shows a populated list rather than empty state.
func seedVisualInvoices(t *testing.T, db *store.DB, settings model.Settings) {
	t.Helper()
	rows := []struct {
		number   string
		customer string
		daysAgo  int
		rate     string
		typ      model.InvoiceType
	}{
		{"AEX2425-4", "Northwind Trading Co.", 3, "4850.00", model.InvoiceExportLUT},
		{"AEX2425-3", "Contoso Retail Pvt Ltd", 21, "3200.00", model.InvoiceExportLUT},
		{"DOM2425-2", "Fabrikam Systems", 34, "1750.00", model.InvoiceDomestic},
		{"AEXI2425-1", "Northwind Trading Co.", 52, "6100.00", model.InvoiceExportIGST},
	}
	for _, r := range rows {
		st := NewEditorState(settings)
		st.SetType(r.typ)
		st.Number = r.number
		st.Date = time.Now().AddDate(0, 0, -r.daysAgo)
		st.Customer.Name = r.customer
		st.Customer.BillingAddress = []string{"1209 Orange Street", "Wilmington, DE 19801"}
		st.PlaceOfSupply = "97-Other Territory"
		st.Items[0].Description = "Software development services"
		st.Items[0].RateUSD = r.rate
		if r.typ == model.InvoiceDomestic {
			st.Items[0].RateUSD = r.rate
		}
		inv, err := st.Build()
		if err != nil {
			t.Fatalf("build seed invoice %s: %v", r.number, err)
		}
		if _, err := db.SaveInvoice(*inv); err != nil {
			t.Fatalf("save seed invoice %s: %v", r.number, err)
		}
	}
}

// TestVisual_Screens captures every screen of the app under the real
// application theme so layout and colour changes can be reviewed visually.
// Run with: go test -run TestVisual_Screens ./internal/ui -v
func TestVisual_Screens(t *testing.T) {
	fyneApp := test.NewTempApp(t)
	fyneApp.Settings().SetTheme(NewTheme())

	settings := visualSettings()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.SaveSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	seedVisualInvoices(t, db, settings)
	for _, c := range visualMasters().Customers {
		if _, err := db.SaveCustomer(c); err != nil {
			t.Fatalf("seed customer: %v", err)
		}
	}
	for _, it := range visualMasters().Items {
		if _, err := db.SaveItem(it); err != nil {
			t.Fatalf("seed item: %v", err)
		}
	}

	a, err := NewAppWithFyneApp(fyneApp, db, "")
	if err != nil {
		t.Fatalf("NewAppWithFyneApp: %v", err)
	}

	const w, h = 1280, 860

	ed := NewEditor(visualEditorState(settings), settings, visualMasters(), func(model.Invoice) error { return nil }, func() {})
	shoot(t, "editor", fyne.NewSize(w, h), ed.Container())
	shoot(t, "invoices", fyne.NewSize(w, h), a.invoicesScreen.Container())
	shoot(t, "settings", fyne.NewSize(w, h), a.buildSettingsContainer())
	shoot(t, "masters", fyne.NewSize(w, h), NewMastersScreen(a.win, db).Container())
	shoot(t, "about", fyne.NewSize(w, h), NewAboutScreen(a.win, fyneApp, db, "").Container())
	shoot(t, "shell", fyne.NewSize(w, h), a.tabs)

	// A domestic invoice, where the export-only supply fields drop away and
	// the tax splits into CGST + SGST.
	domState := visualEditorState(settings)
	domState.ApplySettings(settings)
	domState.SetType(model.InvoiceDomestic)
	domState.Number = "DOM2425-3"
	domCust := testfixture.DomesticCustomerRecord()
	domState.Customer.Name = domCust.Name
	domState.Customer.GSTIN = domCust.GSTIN
	domState.Customer.BillingAddress = domCust.BillingAddress
	domState.Customer.ShippingAddress = domCust.ShippingAddress
	domState.PlaceOfSupply = "33-Tamil Nadu"
	domState.Items = domState.Items[:1]
	domState.Items[0].Description = "Software development services"
	domState.Items[0].RateUSD = "100000.00"
	domEd := NewEditor(domState, settings, visualMasters(), func(model.Invoice) error { return nil }, func() {})
	shoot(t, "editor-domestic", fyne.NewSize(w, 1100), domEd.Container())

	// A tall capture of the editor so the whole document sheet — including the
	// line-item grid below the fold at normal height — is reviewable.
	edTall := NewEditor(visualEditorState(settings), settings, visualMasters(), func(model.Invoice) error { return nil }, func() {})
	shoot(t, "editor-full", fyne.NewSize(w, 1500), edTall.Container())
}
