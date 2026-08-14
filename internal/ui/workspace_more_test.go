package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"dock-invoice/internal/model"
	"dock-invoice/internal/org"
	"dock-invoice/internal/store"
)

// withRedirectedAppDir is defined in testenv_test.go.

// TestOpenWorkspace_FreshInstall covers 3.1: an empty data directory gets
// exactly one organization, named "My Organization", pointing at
// invoicer.db, and it is opened.
func TestOpenWorkspace_FreshInstall(t *testing.T) {
	dir := withRedirectedAppDir(t)

	ws, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer ws.Close()

	if ws.Orgs == nil {
		t.Fatal("expected registry mode (Orgs != nil) with no INVOICER_DB_PATH set")
	}
	if ws.Orgs.Len() != 1 {
		t.Fatalf("Orgs.Len() = %d, want 1", ws.Orgs.Len())
	}
	active, ok := ws.Orgs.Active()
	if !ok {
		t.Fatal("no active organization after a fresh install")
	}
	if active.Name != DefaultOrgName {
		t.Errorf("Name = %q, want %q", active.Name, DefaultOrgName)
	}
	if active.DBFile != LegacyDBFileName {
		t.Errorf("DBFile = %q, want %q", active.DBFile, LegacyDBFileName)
	}
	if ws.Path != filepath.Join(dir, LegacyDBFileName) {
		t.Errorf("Path = %q, want %q", ws.Path, filepath.Join(dir, LegacyDBFileName))
	}
	if _, err := os.Stat(filepath.Join(dir, org.RegistryFileName)); err != nil {
		t.Errorf("organizations.json was not created: %v", err)
	}
}

// TestOpenWorkspace_UpgradePreservesExistingData is the highest-risk case
// (3.2): a pre-existing invoicer.db with settings, a customer, an item and
// two invoices must be adopted in place — not moved, not copied — with every
// record still readable, and named after the configured company rather than
// the generic default.
func TestOpenWorkspace_UpgradePreservesExistingData(t *testing.T) {
	dir := withRedirectedAppDir(t)
	legacyPath := filepath.Join(dir, LegacyDBFileName)

	seedDB, err := store.Open(legacyPath)
	if err != nil {
		t.Fatalf("seed store.Open: %v", err)
	}
	if err := seedDB.SaveSettings(model.Settings{
		Company: model.Company{Name: "Established Traders LLP", GSTIN: "33AAAAA0000A1Z5"},
	}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if _, err := seedDB.SaveCustomer(model.Customer{Name: "Loyal Customer"}); err != nil {
		t.Fatalf("SaveCustomer: %v", err)
	}
	if _, err := seedDB.SaveItem(model.Item{Description: "Consulting", HSNSAC: "998314"}); err != nil {
		t.Fatalf("SaveItem: %v", err)
	}
	seedInvoice(t, seedDB, "OLD-1", "Loyal Customer", time.Now().AddDate(0, 0, -10))
	seedInvoice(t, seedDB, "OLD-2", "Loyal Customer", time.Now())
	info, err := os.Stat(legacyPath)
	if err != nil {
		t.Fatalf("stat legacy db before upgrade: %v", err)
	}
	sizeBefore := info.Size()
	if err := seedDB.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	ws, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer ws.Close()

	// Not moved: same path, and the file's on-disk size did not shrink to
	// zero or otherwise get replaced by a fresh empty schema (a copy/move
	// bug would either leave the original in place unmodified — same
	// bytes — or the resolved path would land somewhere else entirely).
	if ws.Path != legacyPath {
		t.Fatalf("Path = %q, want the untouched original location %q", ws.Path, legacyPath)
	}
	if info, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("stat legacy db after upgrade: %v", err)
	} else if info.Size() < sizeBefore {
		t.Errorf("legacy db shrank from %d to %d bytes across the upgrade", sizeBefore, info.Size())
	}

	active, ok := ws.Orgs.Active()
	if !ok {
		t.Fatal("no active organization after upgrade")
	}
	if active.Name != "Established Traders LLP" {
		t.Errorf("adopted org Name = %q, want the configured company name", active.Name)
	}

	invs, err := ws.DB.ListInvoices()
	if err != nil {
		t.Fatalf("ListInvoices: %v", err)
	}
	if len(invs) != 2 {
		t.Fatalf("ListInvoices returned %d, want 2", len(invs))
	}
	custs, err := ws.DB.ListCustomers()
	if err != nil {
		t.Fatalf("ListCustomers: %v", err)
	}
	if len(custs) != 1 || custs[0].Name != "Loyal Customer" {
		t.Errorf("ListCustomers = %+v, want one Loyal Customer", custs)
	}
	items, err := ws.DB.ListItems()
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 || items[0].Description != "Consulting" {
		t.Errorf("ListItems = %+v, want one Consulting item", items)
	}
}

// TestOpenWorkspace_UpgradeWithNoCompanyName covers 3.3 end-to-end through
// OpenWorkspace (registry_test.go / orgs_test.go only exercise legacyOrgName
// directly, not the full startup path): a legacy db with no company name
// configured falls back to the default org name.
func TestOpenWorkspace_UpgradeWithNoCompanyName(t *testing.T) {
	dir := withRedirectedAppDir(t)
	legacyPath := filepath.Join(dir, LegacyDBFileName)

	seedDB, err := store.Open(legacyPath)
	if err != nil {
		t.Fatalf("seed store.Open: %v", err)
	}
	// A settings row exists, but Company.Name is blank.
	if err := seedDB.SaveSettings(model.Settings{Company: model.Company{City: "Chennai"}}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if err := seedDB.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	ws, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer ws.Close()

	active, ok := ws.Orgs.Active()
	if !ok {
		t.Fatal("no active organization")
	}
	if active.Name != DefaultOrgName {
		t.Errorf("Name = %q, want fallback %q", active.Name, DefaultOrgName)
	}
}

// TestOpenWorkspace_UpgradeWithUnreadableLegacyDB covers 3.4: a corrupt
// invoicer.db must not panic startup. Records which of the two acceptable
// outcomes (naming falls back to the default but the app still starts, vs.
// store.Open itself fails outright) actually happens.
func TestOpenWorkspace_UpgradeWithUnreadableLegacyDB(t *testing.T) {
	dir := withRedirectedAppDir(t)
	legacyPath := filepath.Join(dir, LegacyDBFileName)
	if err := os.WriteFile(legacyPath, []byte("this is not a sqlite database, just garbage bytes"), 0o644); err != nil {
		t.Fatalf("seed garbage legacy db: %v", err)
	}

	ws, err := OpenWorkspace() // must not panic regardless of outcome
	if err != nil {
		t.Logf("OpenWorkspace failed outright on a corrupt legacy db (acceptable outcome): %v", err)
		return
	}
	defer ws.Close()

	active, ok := ws.Orgs.Active()
	if !ok {
		t.Fatal("OpenWorkspace succeeded but left no active organization")
	}
	t.Logf("OpenWorkspace succeeded over a corrupt legacy db; adopted org name = %q", active.Name)
	if active.Name != DefaultOrgName {
		t.Errorf("Name = %q, want fallback %q when the legacy db can't be read for its company name", active.Name, DefaultOrgName)
	}
}

// TestOpenWorkspace_EnvOverride_NoRegistryArtifacts strengthens 3.5's
// existing coverage (TestOpenWorkspace_EnvOverrideIsSingleDatabaseMode in
// orgs_test.go checks ws.Orgs == nil and the pinned path, but never checks
// that single-database mode leaves the app data directory alone, or that the
// switcher is actually hidden for an App built over that workspace).
func TestOpenWorkspace_EnvOverride_NoRegistryArtifacts(t *testing.T) {
	appDataDir := withRedirectedAppDir(t)
	pinned := filepath.Join(t.TempDir(), "pinned.db")
	t.Setenv(EnvDBPath, pinned)

	ws, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer ws.Close()

	if ws.Orgs != nil {
		t.Fatal("expected single-database mode")
	}
	if _, err := os.Stat(filepath.Join(appDataDir, org.RegistryFileName)); !os.IsNotExist(err) {
		t.Errorf("organizations.json was created in the app data directory despite INVOICER_DB_PATH being set; stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDataDir, LegacyDBFileName)); !os.IsNotExist(err) {
		t.Errorf("invoicer.db was created in the app data directory despite INVOICER_DB_PATH being set; stat error = %v", err)
	}

	a, err := NewWorkspaceAppWithFyneApp(test.NewTempApp(t), ws)
	if err != nil {
		t.Fatalf("NewWorkspaceAppWithFyneApp: %v", err)
	}
	if orgsButton(a) != nil {
		t.Error("the organization UI should be hidden when the workspace came from INVOICER_DB_PATH")
	}
}

// TestOpenWorkspace_WhitespaceEnvPathTreatedAsUnset covers 3.6: envDBPath's
// TrimSpace claim must actually hold through OpenWorkspace, not just in
// isolation — a whitespace-only INVOICER_DB_PATH must fall through to
// registry mode rather than resolving to the current directory.
func TestOpenWorkspace_WhitespaceEnvPathTreatedAsUnset(t *testing.T) {
	withRedirectedAppDir(t)
	t.Setenv(EnvDBPath, "   ")

	ws, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer ws.Close()

	if ws.Orgs == nil {
		t.Fatal("whitespace INVOICER_DB_PATH activated single-database mode; want it treated as unset")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if filepath.Dir(ws.Path) == cwd {
		t.Errorf("Path = %q resolved relative to the current directory", ws.Path)
	}
}

// TestOpenWorkspace_ReopenPersistsSwitchedOrg covers 3.7: after switching to
// a second organization and reopening the workspace fresh (simulating an app
// restart), OpenWorkspace must come back up on the second organization, not
// silently revert to the first.
func TestOpenWorkspace_ReopenPersistsSwitchedOrg(t *testing.T) {
	dir := withRedirectedAppDir(t)

	ws1, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace (first run): %v", err)
	}
	first, _ := ws1.Orgs.Active()
	second, err := ws1.Orgs.Add("Second Business")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := ws1.Orgs.SetActive(second.ID); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if err := ws1.Close(); err != nil {
		t.Fatalf("close first workspace: %v", err)
	}

	ws2, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace (restart): %v", err)
	}
	defer ws2.Close()

	active, ok := ws2.Orgs.Active()
	if !ok {
		t.Fatal("no active organization after restart")
	}
	if active.ID != second.ID {
		t.Errorf("reopened active org = %q (%q), want %q (%q)", active.ID, active.Name, second.ID, second.Name)
	}
	if active.ID == first.ID {
		t.Error("restart reverted to the first organization instead of the one switched to")
	}
	_ = dir
}

// TestOpenWorkspace_MissingDBFileIsRecreated covers 3.8: a user (or an
// external backup tool) deleting an organization's .db file between launches
// must not crash the next startup.
func TestOpenWorkspace_MissingDBFileIsRecreated(t *testing.T) {
	withRedirectedAppDir(t)

	ws1, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace (first run): %v", err)
	}
	path := ws1.Path
	if err := ws1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("simulate user deleting the db file: %v", err)
	}

	ws2, err := OpenWorkspace()
	if err != nil {
		t.Fatalf("OpenWorkspace with a missing db file returned an error (crash risk on next launch): %v", err)
	}
	defer ws2.Close()
	t.Log("store.Open recreated the missing database file rather than erroring")

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected the db file to be recreated at %q: %v", path, err)
	}
	// The recreated database must be a normal, empty, usable database — not
	// a zero-byte file the app then chokes on.
	if _, err := ws2.DB.ListInvoices(); err != nil {
		t.Errorf("recreated database is not queryable: %v", err)
	}
}
