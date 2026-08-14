package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/model"
	"dock-invoice/internal/org"
	"dock-invoice/internal/store"
)

// newMultiOrgTestApp builds a headless App backed by a real on-disk
// organization registry with two organizations, and returns the app plus the
// registry directory.
func newMultiOrgTestApp(t *testing.T) (*App, *org.Registry) {
	t.Helper()
	dir := t.TempDir()

	reg, err := org.Load(dir)
	if err != nil {
		t.Fatalf("org.Load: %v", err)
	}
	if _, err := reg.Add("First Business"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := reg.Add("Second Business"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	path, ok := reg.ActivePath()
	if !ok {
		t.Fatal("registry has no active organization")
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	a, err := NewWorkspaceAppWithFyneApp(test.NewTempApp(t), &Workspace{DB: db, Path: path, Orgs: reg})
	if err != nil {
		t.Fatalf("NewWorkspaceAppWithFyneApp: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a, reg
}

// TestSwitchOrg_IsolatesInvoices is the guarantee the whole file-per-org
// design exists for: an invoice saved under one organization must be invisible
// under another.
func TestSwitchOrg_IsolatesInvoices(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]

	seedInvoice(t, a.db, "FIRST-1", "Customer A", time.Now())

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	invs, err := a.db.ListInvoices()
	if err != nil {
		t.Fatalf("ListInvoices: %v", err)
	}
	if len(invs) != 0 {
		t.Fatalf("second organization sees %d invoice(s) from the first: %+v", len(invs), invs)
	}

	seedInvoice(t, a.db, "SECOND-1", "Customer B", time.Now())

	if err := a.switchOrg(first.ID); err != nil {
		t.Fatalf("switchOrg back: %v", err)
	}
	invs, err = a.db.ListInvoices()
	if err != nil {
		t.Fatalf("ListInvoices: %v", err)
	}
	if len(invs) != 1 || invs[0].Number != "FIRST-1" {
		t.Fatalf("first organization should still hold exactly FIRST-1, got %+v", invs)
	}
}

// TestSwitchOrg_IsolatesSettings pins that each organization carries its own
// company/GSTIN — the whole point of running two businesses from one app.
func TestSwitchOrg_IsolatesSettings(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]

	s := a.settings
	s.Company.Name = "First Business Pvt Ltd"
	s.Company.GSTIN = "33AAAAA0000A1Z5"
	if err := a.saveSettings(s); err != nil {
		t.Fatalf("saveSettings: %v", err)
	}

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	if got := a.settings.Company.GSTIN; got != "" {
		t.Errorf("second organization inherited GSTIN %q from the first", got)
	}
	// A fresh organization's database is a first run, so it must come up
	// with seeded defaults rather than an empty settings record.
	if a.settings.Defaults.HSNSAC == "" || len(a.settings.NumberPatterns) == 0 {
		t.Errorf("new organization did not get seeded defaults: %+v", a.settings.Defaults)
	}

	if err := a.switchOrg(first.ID); err != nil {
		t.Fatalf("switchOrg back: %v", err)
	}
	if got := a.settings.Company.GSTIN; got != "33AAAAA0000A1Z5" {
		t.Errorf("first organization's GSTIN = %q, want it preserved", got)
	}
}

// TestSwitchOrg_PersistsActiveSelection covers relaunch: the app must reopen
// on the organization the user was last working in.
func TestSwitchOrg_PersistsActiveSelection(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	second := reg.List()[1]

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}

	reloaded, err := org.Load(reg.Dir())
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}
	if reloaded.ActiveID != second.ID {
		t.Errorf("persisted ActiveID = %q, want %q", reloaded.ActiveID, second.ID)
	}
}

// TestSwitchOrg_RebuildsScreensOverTheNewDatabase guards the bug that would
// otherwise be invisible: swapping a.db but leaving the Invoices screen bound
// to the old handle, so the UI keeps showing the previous organization's data.
func TestSwitchOrg_RebuildsScreensOverTheNewDatabase(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	second := reg.List()[1]

	seedInvoice(t, a.db, "FIRST-1", "Customer A", time.Now())
	before := a.invoicesScreen

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	if a.invoicesScreen == before {
		t.Fatal("InvoicesScreen was not rebuilt; it is still bound to the previous organization's database")
	}
	if a.dbPath != reg.Path(second) {
		t.Errorf("dbPath = %q, want %q", a.dbPath, reg.Path(second))
	}
	if title := a.win.Title(); !strings.Contains(title, second.Name) {
		t.Errorf("window title = %q, want it to name %q", title, second.Name)
	}
}

// TestSwitchOrg_UnknownIDLeavesAppUntouched: a failed switch must not strand
// the app with a closed or missing database.
func TestSwitchOrg_UnknownIDLeavesAppUntouched(t *testing.T) {
	a, _ := newMultiOrgTestApp(t)
	pathBefore := a.dbPath

	if err := a.switchOrg("does-not-exist"); err == nil {
		t.Fatal("expected an error for an unknown organization id")
	}
	if a.dbPath != pathBefore {
		t.Errorf("dbPath changed to %q after a failed switch", a.dbPath)
	}
	// The original database must still be usable.
	if _, err := a.db.ListInvoices(); err != nil {
		t.Errorf("database unusable after a failed switch: %v", err)
	}
}

// TestSwitchOrg_SameOrgIsANoOp keeps the switcher's OnChanged from tearing
// down and rebuilding the world when the selection has not actually moved.
func TestSwitchOrg_SameOrgIsANoOp(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	active, ok := reg.Active()
	if !ok {
		t.Fatal("no active organization")
	}
	before := a.invoicesScreen

	if err := a.switchOrg(active.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	if a.invoicesScreen != before {
		t.Error("switching to the already-open organization rebuilt the screens")
	}
}

// orgsButton returns the organization bar's button, or nil when the window
// shows none. Unlike findButtonByText it does not fail the test on a miss:
// its absence is exactly what the single-database-mode tests assert.
func orgsButton(a *App) *widget.Button {
	var found *widget.Button
	walk(a.win.Content(), func(o any) {
		if found != nil {
			return
		}
		if b, ok := o.(*widget.Button); ok && b.Text == "Organizations…" {
			found = b
		}
	})
	return found
}

func TestApp_SingleDatabaseModeHasNoOrgUI(t *testing.T) {
	a := newTestApp(t)
	if orgsButton(a) != nil {
		t.Error("single-database mode should not offer an Organizations button")
	}
	if err := a.switchOrg("anything"); err == nil {
		t.Error("switchOrg should fail without an organization registry")
	}
	if got := a.win.Title(); got != "Invoices" {
		t.Errorf("window title = %q, want %q", got, "Invoices")
	}
}

// TestOrgBar_ButtonOpensManageDialog covers the only route into the
// organization UI: if this button stops reaching the dialog, a
// multi-organization user cannot change organization at all.
func TestOrgBar_ButtonOpensManageDialog(t *testing.T) {
	a, _ := newMultiOrgTestApp(t)

	btn := orgsButton(a)
	if btn == nil {
		t.Fatal("multi-organization mode should offer an Organizations button")
	}
	if a.win.Canvas().Overlays().Top() != nil {
		t.Fatal("an overlay was already showing before the button was tapped")
	}

	test.Tap(btn)

	if a.win.Canvas().Overlays().Top() == nil {
		t.Fatal("tapping Organizations… did not open the manage dialog")
	}
}

func TestRename_ReflectedInWindowTitle(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	active, _ := reg.Active()

	if err := reg.Rename(active.ID, "Renamed Business"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	a.applyWindowTitle()

	if !strings.Contains(a.win.Title(), "Renamed Business") {
		t.Errorf("window title = %q, want it to name the renamed organization", a.win.Title())
	}
}

// TestOpenWorkspace_EnvOverrideIsSingleDatabaseMode: INVOICER_DB_PATH names
// one specific file, so it must bypass the registry entirely.
func TestOpenWorkspace_EnvOverrideIsSingleDatabaseMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pinned.db")
	t.Setenv(EnvDBPath, path)

	ws, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer ws.Close()

	if ws.Orgs != nil {
		t.Error("INVOICER_DB_PATH must not activate multi-organization mode")
	}
	if ws.Path != path {
		t.Errorf("Path = %q, want %q", ws.Path, path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("database was not created at the pinned path: %v", err)
	}
}

// TestLegacyOrgName_BorrowsConfiguredCompanyName covers the upgrade path: an
// existing install already said who it is, so the adopted organization should
// carry that name rather than a generic placeholder.
func TestLegacyOrgName_BorrowsConfiguredCompanyName(t *testing.T) {
	dir := t.TempDir()

	db, err := store.Open(filepath.Join(dir, LegacyDBFileName))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := db.SaveSettings(model.Settings{
		Company: model.Company{Name: "Rajender Electricals"},
	}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	db.Close()

	reg, err := org.Load(dir)
	if err != nil {
		t.Fatalf("org.Load: %v", err)
	}
	if got := legacyOrgName(reg); got != "Rajender Electricals" {
		t.Errorf("legacyOrgName = %q, want %q", got, "Rajender Electricals")
	}
}

// TestLegacyOrgName_FallsBackWhenNothingConfigured covers a genuinely fresh
// install, where there is no company name to borrow.
func TestLegacyOrgName_FallsBackWhenNothingConfigured(t *testing.T) {
	reg, err := org.Load(t.TempDir())
	if err != nil {
		t.Fatalf("org.Load: %v", err)
	}
	if got := legacyOrgName(reg); got != DefaultOrgName {
		t.Errorf("legacyOrgName = %q, want %q", got, DefaultOrgName)
	}
}

// TestAdoptLegacy_UpgradeKeepsExistingInvoices is the end-to-end upgrade
// check: an install that already has invoices in invoicer.db must find them
// under the adopted organization, with the file left where it was.
func TestAdoptLegacy_UpgradeKeepsExistingInvoices(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, LegacyDBFileName)

	db, err := store.Open(legacy)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	seedInvoice(t, db, "OLD-1", "Existing Customer", time.Now())
	db.Close()

	reg, err := org.Load(dir)
	if err != nil {
		t.Fatalf("org.Load: %v", err)
	}
	if _, err := reg.AdoptLegacy(legacyOrgName(reg), LegacyDBFileName); err != nil {
		t.Fatalf("AdoptLegacy: %v", err)
	}

	path, ok := reg.ActivePath()
	if !ok {
		t.Fatal("no active organization after adoption")
	}
	if path != legacy {
		t.Fatalf("active path = %q, want the untouched legacy file %q", path, legacy)
	}

	reopened, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	invs, err := reopened.ListInvoices()
	if err != nil {
		t.Fatalf("ListInvoices: %v", err)
	}
	if len(invs) != 1 || invs[0].Number != "OLD-1" {
		t.Fatalf("existing invoices lost on upgrade: %+v", invs)
	}
}

// TestOrgBar_OverlayDoesNotBlockTabClicks is the risk the overlay introduces:
// the Organizations button is laid over the tab strip's row, and if the
// container carrying it intercepted taps, the tabs underneath would stop
// responding — the app would be unusable and no other test would notice.
// Plain containers are not tappable in Fyne, so the empty area should fall
// through to the tabs; this drives a real canvas tap to prove it.
func TestOrgBar_OverlayDoesNotBlockTabClicks(t *testing.T) {
	a, _ := newMultiOrgTestApp(t)

	win := test.NewTempWindow(t, a.win.Content())
	win.Resize(fyne.NewSize(1280, 860))
	win.Show()

	if got := a.tabs.SelectedIndex(); got != 0 {
		t.Fatalf("test setup: selected tab = %d, want 0", got)
	}
	// The strip's height is where the first tab's content begins.
	stripH := a.tabs.Items[0].Content.Position().Y
	if stripH <= 0 {
		t.Fatalf("could not determine tab strip height (got %v)", stripH)
	}

	// Tap the second tab ("Editor"), on the same row as the button but far
	// from it.
	editorTabCentre := fyne.NewPos(180, stripH/2)
	test.TapCanvas(win.Canvas(), editorTabCentre)

	if got := a.tabs.SelectedIndex(); got == 0 {
		t.Error("tapping a tab on the organization button's row did nothing — the overlay is swallowing taps meant for the tab strip")
	}
}
