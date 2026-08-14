package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/model"
	"dock-invoice/internal/store"
	"dock-invoice/internal/testfixture"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	fyneApp := test.NewTempApp(t)
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// NewAppWithFyneApp (not NewApp) — NewApp creates a real, e.g.
	// glfw-backed, application, which would try to open an actual OS
	// window and hang here. Tests must reuse the headless test app instead.
	a, err := NewAppWithFyneApp(fyneApp, db, "")
	if err != nil {
		t.Fatalf("NewAppWithFyneApp: %v", err)
	}
	return a
}

// TestApp_BuildsAllFourScreensWithoutPanicking is a smoke test over the
// full wiring (Invoices/Editor/Settings/Masters tabs, seeded default
// settings, an empty database). This is the kind of test that would have
// caught the nil-pointer build-ordering bug in Editor.build() at the
// integration level, not just within the editor's own unit tests.
func TestApp_BuildsAllFourScreensWithoutPanicking(t *testing.T) {
	a := newTestApp(t)
	if a.tabs == nil || len(a.tabs.Items) != 5 {
		t.Fatalf("expected 5 tabs, got %+v", a.tabs)
	}
	names := []string{"Invoices", "Editor", "Settings", "Masters", "About"}
	for i, want := range names {
		if got := a.tabs.Items[i].Text; got != want {
			t.Errorf("tab %d = %q, want %q", i, got, want)
		}
	}
}

// TestApp_New_AssignsNumberAndOpensEditor exercises the InvoicesScreen's
// "New" button end to end: it should allocate an invoice number from the
// configured series (default settings seed a pattern for every type) and
// switch to the Editor tab with that number pre-filled.
func TestApp_New_AssignsNumberAndOpensEditor(t *testing.T) {
	a := newTestApp(t)

	newBtn := findButtonByText(t, a.invoicesScreen.Container(), "New invoice")
	test.Tap(newBtn)

	if a.tabs.Selected() != a.editorTab {
		t.Fatal("expected New to switch to the Editor tab")
	}

	numberEntry := findEntryByPlaceholderOrFirst(t, a.editorTab.Content)
	if numberEntry == nil || numberEntry.Text == "" {
		t.Error("expected the new invoice to have an allocated number")
	}
}

// TestApp_SaveFromEditor_PersistsAndReturnsToInvoices exercises the full
// New -> fill in required fields -> Save -> back to Invoices flow against a
// real (in-memory) database.
func TestApp_SaveFromEditor_PersistsAndReturnsToInvoices(t *testing.T) {
	a := newTestApp(t)

	state := NewEditorState(a.settings)
	state.Number = "AEX2425-1"
	state.Customer.Name = "Test Customer"
	state.Items[0].RateUSD = "100.00"
	a.openEditor(state)

	if a.tabs.Selected() != a.editorTab {
		t.Fatal("openEditor should switch to the Editor tab")
	}

	saveBtn := findButtonByText(t, a.editorTab.Content, "Save & export PDF")
	test.Tap(saveBtn)

	if a.tabs.Selected() != a.invoicesTab {
		t.Error("Save should switch back to the Invoices tab on success")
	}

	invs, err := a.db.ListInvoices()
	if err != nil {
		t.Fatalf("ListInvoices: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected 1 saved invoice, got %d", len(invs))
	}
	if invs[0].Customer.Name != "Test Customer" {
		t.Errorf("saved invoice customer = %q, want Test Customer", invs[0].Customer.Name)
	}
}

// TestApp_SaveFromEditor_AlsoRendersPDF is the regression test for bug A1:
// saving an invoice from the Editor must also produce its PDF (the
// "implicit re-export-on-save" described in the README), not just persist
// the row to the database. Before the fix, openEditor's onSave callback
// never called a.renderAndSave at all.
func TestApp_SaveFromEditor_AlsoRendersPDF(t *testing.T) {
	a := newTestApp(t)
	outDir := t.TempDir()
	a.settings.OutputDir = outDir

	state := NewEditorState(a.settings)
	state.Number = "AEX2425-1"
	state.Customer.Name = "Test Customer"
	state.Items[0].RateUSD = "100.00"
	a.openEditor(state)

	saveBtn := findButtonByText(t, a.editorTab.Content, "Save & export PDF")
	test.Tap(saveBtn)

	wantPath := filepath.Join(outDir, "AEX2425-1.pdf")
	if info, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected a PDF at %s after Save, stat error: %v", wantPath, err)
	} else if info.Size() == 0 {
		t.Errorf("PDF at %s is empty", wantPath)
	}
}

// TestApp_SaveFromEditor_PDFFailureStillKeepsTheSavedInvoice covers the
// other half of bug A1's fix: if the DB save succeeds but the PDF
// render/write fails afterwards, the invoice must stay saved (not be
// rolled back or hidden from the user) and the editor should still return
// to the Invoices tab, since the save itself did succeed.
func TestApp_SaveFromEditor_PDFFailureStillKeepsTheSavedInvoice(t *testing.T) {
	a := newTestApp(t)
	// Point OutputDir at a path whose parent segment is a plain file, not
	// a directory, so the os.MkdirAll inside renderAndSave is guaranteed
	// to fail regardless of the host filesystem's permissions.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	a.settings.OutputDir = filepath.Join(blocker, "invoices")

	state := NewEditorState(a.settings)
	state.Number = "AEX2425-2"
	state.Customer.Name = "Test Customer"
	state.Items[0].RateUSD = "100.00"
	a.openEditor(state)

	saveBtn := findButtonByText(t, a.editorTab.Content, "Save & export PDF")
	test.Tap(saveBtn)

	invs, err := a.db.ListInvoices()
	if err != nil {
		t.Fatalf("ListInvoices: %v", err)
	}
	if len(invs) != 1 {
		t.Fatalf("expected the invoice to remain saved despite the PDF failure, got %d rows", len(invs))
	}
	if a.tabs.Selected() != a.invoicesTab {
		t.Error("expected Save to still switch to the Invoices tab when only the PDF export fails")
	}
}

// TestDefaultOutputDir_UnderHomeDocuments is the regression test for bug
// A2: the fallback output directory must be a real, writable, user-visible
// location, not "." (which resolves to "/" inside a double-clicked,
// packaged macOS .app and fails with a confusing permission error).
func TestDefaultOutputDir_UnderHomeDocuments(t *testing.T) {
	home := withRedirectedHome(t)

	got, err := defaultOutputDir()
	if err != nil {
		t.Fatalf("defaultOutputDir: %v", err)
	}
	want := filepath.Join(home, "Documents", "Invoices")
	if got != want {
		t.Errorf("defaultOutputDir() = %q, want %q", got, want)
	}
}

// TestApp_RenderAndSave_UsesDefaultOutputDirWhenUnset exercises the actual
// renderAndSave fallback path (not just defaultOutputDir in isolation),
// confirming a PDF lands under <home>/Documents/Invoices when
// settings.OutputDir is empty, and that the directory is created on
// demand.
func TestApp_RenderAndSave_UsesDefaultOutputDirWhenUnset(t *testing.T) {
	a := newTestApp(t)
	home := withRedirectedHome(t)
	a.settings.OutputDir = "" // exercise the fallback, not an explicit setting

	state := NewEditorState(a.settings)
	state.Number = "DOM2425-1"
	state.Customer.Name = "Test Customer"
	state.Items[0].RateUSD = "100.00"
	inv, err := state.Build()
	if err != nil {
		t.Fatalf("state.Build: %v", err)
	}

	path, err := a.renderAndSave(*inv)
	if err != nil {
		t.Fatalf("renderAndSave: %v", err)
	}
	wantPath := filepath.Join(home, "Documents", "Invoices", "DOM2425-1.pdf")
	if path != wantPath {
		t.Errorf("renderAndSave path = %q, want %q", path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("expected a PDF at %s, stat error: %v", wantPath, err)
	}
}

// TestNewAppWithFyneApp_PersistsDefaultSettingsOnFirstRun is the
// regression test for bug A3: on a first run against an empty database,
// the seeded default settings must be written to the DB immediately, not
// just held in memory until "Save Settings" is pressed.
func TestNewAppWithFyneApp_PersistsDefaultSettingsOnFirstRun(t *testing.T) {
	fyneApp := test.NewTempApp(t)
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, ok, err := db.GetSettings(); err != nil {
		t.Fatalf("GetSettings (pre): %v", err)
	} else if ok {
		t.Fatal("test setup: expected no settings row before NewAppWithFyneApp")
	}

	a, err := NewAppWithFyneApp(fyneApp, db, "")
	if err != nil {
		t.Fatalf("NewAppWithFyneApp: %v", err)
	}

	persisted, ok, err := db.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings (post): %v", err)
	}
	if !ok {
		t.Fatal("expected the seeded default settings to be persisted on first run, found none")
	}
	if persisted.NumberPatterns[model.InvoiceExportLUT] != a.settings.NumberPatterns[model.InvoiceExportLUT] {
		t.Errorf("persisted NumberPatterns[export_lut] = %q, want %q",
			persisted.NumberPatterns[model.InvoiceExportLUT], a.settings.NumberPatterns[model.InvoiceExportLUT])
	}
	if persisted.LastFXFactor != a.settings.LastFXFactor {
		t.Errorf("persisted LastFXFactor = %q, want %q", persisted.LastFXFactor, a.settings.LastFXFactor)
	}
}

// TestApp_OpenEditor_LoadsMastersPickers is the integration test for the
// deferred Masters-wire-up: customers/items saved under Masters must appear
// in the editor's pickers the next time New/Edit/Clone opens an invoice.
func TestApp_OpenEditor_LoadsMastersPickers(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.db.SaveCustomer(model.Customer{Name: "Wired Customer", GSTIN: "33A"}); err != nil {
		t.Fatalf("SaveCustomer: %v", err)
	}
	if _, err := a.db.SaveItem(model.Item{Description: "Wired Item", HSNSAC: "998314"}); err != nil {
		t.Fatalf("SaveItem: %v", err)
	}

	state := NewEditorState(a.settings)
	state.Number = "AEX-M1"
	a.openEditor(state)

	// openEditor builds a NewEditor with loadEditorMasters(a.db). Peek the
	// snapshot by reconstructing what it must have loaded, and by confirming
	// the picker options surface on the live editor tree.
	loaded := loadEditorMasters(a.db)
	if len(loaded.Customers) != 1 || loaded.Customers[0].Name != "Wired Customer" {
		t.Fatalf("loadEditorMasters customers = %+v, want one Wired Customer", loaded.Customers)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].Description != "Wired Item" {
		t.Fatalf("loadEditorMasters items = %+v, want one Wired Item", loaded.Items)
	}

	var selectLabels [][]string
	walk(a.editorTab.Content, func(o any) {
		if s, ok := o.(*widget.Select); ok {
			selectLabels = append(selectLabels, append([]string(nil), s.Options...))
		}
		// The saved-item picker is a searchComboBox (type-to-filter),
		// not a widget.Select — its option labels live in c.options
		// rather than a public Options field.
		if c, ok := o.(*searchComboBox); ok {
			labels := make([]string, len(c.options))
			for i, opt := range c.options {
				labels[i] = opt.Label
			}
			selectLabels = append(selectLabels, labels)
		}
	})
	foundCustomer, foundItem := false, false
	for _, opts := range selectLabels {
		if containsText(opts, "Wired Customer") {
			foundCustomer = true
		}
		if containsText(opts, "Wired Item") {
			foundItem = true
		}
	}
	if !foundCustomer {
		t.Errorf("editor select options missing \"Wired Customer\"; all selects = %v", selectLabels)
	}
	if !foundItem {
		t.Errorf("editor select options missing \"Wired Item\"; all selects = %v", selectLabels)
	}
}

// TestStampInvoiceFromSettings_FillsSellerAndBank is the regression for
// "Settings are not picked up in the PDF": stampInvoiceFromSettings must copy
// Company GSTIN/State/PAN (and bank) onto the invoice used for render.
func TestStampInvoiceFromSettings_FillsSellerAndBank(t *testing.T) {
	inv := model.Invoice{
		Type:   model.InvoiceExportLUT,
		Number: "AEX-1",
		// Deliberately empty Seller — the bug symptom.
	}
	s := model.Settings{
		Company: testfixture.SellerCompany(),
		LogoPath:             "/tmp/logo.png", // top-level path, merged when Company.LogoPath empty
		Bank:                 model.Bank{AccountNumber: "123", BankName: "SBI"},
		IncludeHSNSACSummary: true,
		LayoutStyle:          model.LayoutClassic,
		Declarations: map[model.InvoiceType]string{
			model.InvoiceExportLUT: "CUSTOM DECLARATION",
		},
	}
	stampInvoiceFromSettings(&inv, s)

	if inv.Seller.GSTIN != testfixture.SellerGSTIN {
		t.Errorf("Seller.GSTIN = %q, want settings GSTIN", inv.Seller.GSTIN)
	}
	if inv.Seller.PAN != testfixture.SellerPAN {
		t.Errorf("Seller.PAN = %q, want settings PAN", inv.Seller.PAN)
	}
	if inv.Seller.StateLabel() != "33-Tamil Nadu" {
		t.Errorf("Seller.StateLabel = %q, want 33-Tamil Nadu", inv.Seller.StateLabel())
	}
	if inv.Seller.LogoPath != "/tmp/logo.png" {
		t.Errorf("Seller.LogoPath = %q, want merged top-level LogoPath", inv.Seller.LogoPath)
	}
	if inv.Bank.AccountNumber != "123" {
		t.Errorf("Bank.AccountNumber = %q, want 123", inv.Bank.AccountNumber)
	}
	if inv.Notes != "CUSTOM DECLARATION" {
		t.Errorf("Notes = %q, want declaration from settings", inv.Notes)
	}
	if !inv.IncludeHSNSACSummary {
		t.Error("IncludeHSNSACSummary = false, want stamped true from settings")
	}
	if inv.LayoutStyle != model.LayoutClassic {
		t.Errorf("LayoutStyle = %q, want classic filled in from empty invoice field", inv.LayoutStyle)
	}
}

// TestStampInvoiceFromSettings_PreservesInvoiceLayoutOverride is the flip
// side of the fill-in case above: LayoutStyle is a per-invoice override (set
// from the editor), unlike Seller/Bank/IncludeHSNSACSummary which always
// follow Settings. A non-empty invoice value must survive the stamp
// unchanged even when Settings disagrees, in both directions.
func TestStampInvoiceFromSettings_PreservesInvoiceLayoutOverride(t *testing.T) {
	classicInvClassicSettings := model.Invoice{LayoutStyle: model.LayoutClassic}
	stampInvoiceFromSettings(&classicInvClassicSettings, model.Settings{LayoutStyle: model.LayoutClassic})
	if classicInvClassicSettings.LayoutStyle != model.LayoutClassic {
		t.Errorf("LayoutStyle = %q, want classic preserved", classicInvClassicSettings.LayoutStyle)
	}

	classicInvModernSettings := model.Invoice{LayoutStyle: model.LayoutClassic}
	stampInvoiceFromSettings(&classicInvModernSettings, model.Settings{LayoutStyle: model.LayoutModern})
	if classicInvModernSettings.LayoutStyle != model.LayoutClassic {
		t.Errorf("LayoutStyle = %q, want invoice's classic override kept despite modern Settings default", classicInvModernSettings.LayoutStyle)
	}

	modernInvClassicSettings := model.Invoice{LayoutStyle: model.LayoutModern}
	stampInvoiceFromSettings(&modernInvClassicSettings, model.Settings{LayoutStyle: model.LayoutClassic})
	if modernInvClassicSettings.LayoutStyle != model.LayoutModern {
		t.Errorf("LayoutStyle = %q, want invoice's modern override kept despite classic Settings default", modernInvClassicSettings.LayoutStyle)
	}
}

// TestApp_RenderAndSave_UsesCurrentSettingsSeller is the end-to-end version:
// even when the invoice snapshot has an empty Seller, renderAndSave stamps
// live settings before writing the PDF (so GSTIN etc. appear on the page).
func TestApp_RenderAndSave_UsesCurrentSettingsSeller(t *testing.T) {
	a := newTestApp(t)
	outDir := t.TempDir()
	a.settings.OutputDir = outDir
	a.settings.Company = testfixture.SellerCompany()

	// Invoice with blank Seller — mimics saves made before Settings were filled.
	inv := model.Invoice{
		Type:     model.InvoiceExportLUT,
		Number:   "AEX-SETTINGS",
		Date:     time.Now(),
		Customer: model.Customer{Name: "Buyer"},
		Currency: "USD",
		Items: []model.LineItem{
			{Description: "Work", Quantity: 100, RateUSD: 10000, TaxRatePct: 0},
		},
	}

	// Prove the stamp that renderAndSave applies would fill letterhead.
	stamped := inv
	stampInvoiceFromSettings(&stamped, a.settings)
	if stamped.Seller.GSTIN != testfixture.SellerGSTIN || stamped.Seller.PAN != testfixture.SellerPAN {
		t.Fatalf("stamp before render left Seller empty: %+v", stamped.Seller)
	}

	path, err := a.renderAndSave(inv)
	if err != nil {
		t.Fatalf("renderAndSave: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat PDF: %v", err)
	}
	if info.Size() == 0 {
		t.Error("rendered PDF is empty")
	}
}

func findEntryByPlaceholderOrFirst(t *testing.T, root any) *widget.Entry {
	t.Helper()
	var found *widget.Entry
	walk(root, func(o any) {
		if found != nil {
			return
		}
		if e, ok := o.(*widget.Entry); ok {
			found = e
		}
	})
	return found
}
