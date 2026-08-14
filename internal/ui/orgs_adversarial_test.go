package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/org"
	"dock-invoice/internal/store"
)

// ---- 6.2: does a.dbPath ever diverge from Remove()'s returned path? -------

// TestPromptRemoveOrg_DBPathAlwaysMatchesRegistryPath is an investigation of
// 6.2's suspicion (promptRemoveOrg decides "was this the open org?" by
// string-comparing a.dbPath to Remove's returned path). Both values are
// always produced by the same Registry.Path(dir, DBFile) using the same
// r.dir, across every code path that sets a.dbPath (OpenWorkspace,
// switchOrg, reopenActive) and every code path that returns a path from
// Remove — so across Add, AdoptLegacy-style seeding, and switching, the
// strings are provably byte-identical within one process. This test is a
// regression guard for that invariant across a representative sequence
// (legacy-style adopted org, an added org, a switch, then removal of the
// currently-open one) rather than a bug reproduction — no divergence was
// found.
func TestPromptRemoveOrg_DBPathAlwaysMatchesRegistryPath(t *testing.T) {
	dir := t.TempDir()
	reg, err := org.Load(dir)
	if err != nil {
		t.Fatalf("org.Load: %v", err)
	}
	legacy, err := reg.AdoptLegacy("Legacy Business", "invoicer.db")
	if err != nil {
		t.Fatalf("AdoptLegacy: %v", err)
	}
	second, err := reg.Add("Second Business")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	db, err := store.Open(reg.Path(legacy))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	a, err := NewWorkspaceAppWithFyneApp(test.NewTempApp(t), &Workspace{DB: db, Path: reg.Path(legacy), Orgs: reg})
	if err != nil {
		t.Fatalf("NewWorkspaceAppWithFyneApp: %v", err)
	}
	t.Cleanup(func() { a.Close() })

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	if a.dbPath != reg.Path(second) {
		t.Fatalf("a.dbPath = %q, reg.Path(second) = %q — already diverged before Remove", a.dbPath, reg.Path(second))
	}

	// Remove the org that is now open (second) and confirm the returned
	// path string-equals a.dbPath, exactly as promptRemoveOrg assumes.
	removedPath, err := reg.Remove(second.ID)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if removedPath != a.dbPath {
		t.Errorf("DIVERGENCE FOUND: Remove returned %q but a.dbPath was %q — promptRemoveOrg's string compare would misidentify this as removing a different org", removedPath, a.dbPath)
	}
}

// ---- 6.4: case-only duplicate names and ByName resolution -----------------

// newCaseDuplicateOrgApp builds an App over a hand-written registry holding
// two organizations whose names differ only by case. Add and Rename both
// reject that, so the only ways to reach this state are a hand-edited
// organizations.json (exactly what this writes) or two racing instances.
func newCaseDuplicateOrgApp(t *testing.T) (*App, *org.Registry) {
	t.Helper()
	dir := t.TempDir()
	body := `{"organizations":[` +
		`{"id":"first","name":"Acme","dbFile":"orgs/first.db"},` +
		`{"id":"second","name":"ACME","dbFile":"orgs/second.db"}` +
		`],"activeId":"first"}`
	if err := os.WriteFile(filepath.Join(dir, org.RegistryFileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "orgs"), 0o755); err != nil {
		t.Fatalf("mkdir orgs: %v", err)
	}
	reg, err := org.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	firstOrg, _ := reg.Get("first")
	db, err := store.Open(reg.Path(firstOrg))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	a, err := NewWorkspaceAppWithFyneApp(test.NewTempApp(t), &Workspace{DB: db, Path: reg.Path(firstOrg), Orgs: reg})
	if err != nil {
		t.Fatalf("NewWorkspaceAppWithFyneApp: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a, reg
}

// buttonsByText collects every button carrying text, in tree order — which for
// the manage dialog is registry order, one "Open" per organization row.
func buttonsByText(root any, text string) []*widget.Button {
	var found []*widget.Button
	walk(root, func(o any) {
		if b, ok := o.(*widget.Button); ok && b.Text == text {
			found = append(found, b)
		}
	})
	return found
}

// TestManageDialog_OpenButton_CaseOnlyDuplicateNames is the regression test
// for D1, retargeted at the manage dialog now that the switcher is gone.
//
// The original defect was that the switcher resolved the selected row through
// Registry.ByName, which is case-insensitive and returns the first match, so
// picking the second of two same-but-for-case names silently opened the first
// organization: wrong database, wrong GSTIN, wrong number series, no error.
// orgRow's Open button closes over the org.Organization value itself rather
// than looking it up by name, which is what makes it immune — this drives the
// rendered button rather than calling confirmOrgSwitch directly, so a future
// refactor that reintroduces a name lookup here would fail this test.
func TestManageDialog_OpenButton_CaseOnlyDuplicateNames(t *testing.T) {
	a, reg := newCaseDuplicateOrgApp(t)

	a.showManageOrgs()
	overlay := a.win.Canvas().Overlays().Top()
	if overlay == nil {
		t.Fatal("manage dialog did not open")
	}

	opens := buttonsByText(overlay, "Open")
	if len(opens) != 2 {
		t.Fatalf("manage dialog has %d Open buttons, want one per organization (2)", len(opens))
	}
	// The first row is the open organization, so its button is disabled;
	// the second row is the one whose name only differs by case.
	if !opens[0].Disabled() {
		t.Error("the open organization's Open button should be disabled")
	}

	test.Tap(opens[1])

	if a.orgs.ActiveID != "second" {
		t.Errorf("tapping the second row's Open switched the app to org %q, want \"second\"", a.orgs.ActiveID)
	}
	secondOrg, _ := reg.Get("second")
	if a.dbPath != reg.Path(secondOrg) {
		t.Errorf("dbPath = %q, want the second organization's path %q", a.dbPath, reg.Path(secondOrg))
	}
}

// TestByName_StillFirstMatch documents that Registry.ByName itself was left
// case-insensitive and first-match on purpose: it is correct for its remaining
// callers, and it was the switcher's use of it as a primary key that was the
// bug. If a future caller needs exactness, it should not come here.
func TestByName_StillFirstMatch(t *testing.T) {
	_, reg := newCaseDuplicateOrgApp(t)

	got, ok := reg.ByName("ACME")
	if !ok {
		t.Fatal("ByName(\"ACME\") found nothing")
	}
	if got.ID != "first" {
		t.Errorf("ByName(\"ACME\").ID = %q, want \"first\" (documented first-match behaviour)", got.ID)
	}
}

// ---- 6.5: pathological names ------------------------------------------------

// TestManageDialog_PathologicalNamesDoNotPanic covers 6.5: a very long name
// and a name containing characters (newlines, tabs, an RTL override) that
// could upset naive label rendering must not crash the manage dialog or the
// switch itself.
func TestManageDialog_PathologicalNamesDoNotPanic(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)

	longName := strings.Repeat("Extremely Long Organization Name ", 20) // ~680 chars
	weirdName := "Right\u2066Left\u2069Override\nAnd\tTabs 名前 🎉"

	if _, err := reg.Add(longName); err != nil {
		t.Fatalf("Add(long name): %v", err)
	}
	weirdOrg, err := reg.Add(weirdName)
	if err != nil {
		t.Fatalf("Add(weird name): %v", err)
	}

	// Rendering the dialog is where a pathological label would blow up.
	a.showManageOrgs()
	overlay := a.win.Canvas().Overlays().Top()
	if overlay == nil {
		t.Fatal("manage dialog did not open")
	}
	if got := len(buttonsByText(overlay, "Open")); got != 4 {
		t.Fatalf("manage dialog shows %d organization rows, want 4", got)
	}

	// And switch to the pathological-name org end to end.
	if err := a.switchOrg(weirdOrg.ID); err != nil {
		t.Fatalf("switchOrg to pathological-name org: %v", err)
	}
	if !strings.Contains(a.win.Title(), weirdOrg.Name) {
		t.Errorf("window title = %q, want it to contain the pathological org name", a.win.Title())
	}
}

// ---- 6.6: read-only data directory, surfaced through the UI --------------

// TestManageOrgsDialog_AddOnReadOnlyDirectorySurfacesError is the UI half of
// 6.6 (the registry half is TestSave_OnReadOnlyDirectoryReturnsError in
// internal/org): confirms addOrgRow's add() reports the failure via
// a.showError rather than swallowing it — the entry must not be silently
// cleared as if the add succeeded.
func TestManageOrgsDialog_AddOnReadOnlyDirectorySurfacesError(t *testing.T) {
	skipUnlessChmodReadOnlyEnforced(t)
	dir := t.TempDir()
	reg, err := org.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	first, err := reg.Add("First Business")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	db, err := store.Open(reg.Path(first))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	a, err := NewWorkspaceAppWithFyneApp(test.NewTempApp(t), &Workspace{DB: db, Path: reg.Path(first), Orgs: reg})
	if err != nil {
		t.Fatalf("NewWorkspaceAppWithFyneApp: %v", err)
	}
	t.Cleanup(func() { a.Close() })

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	a.showManageOrgs()
	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected the Organizations dialog to be showing")
	}
	entry := findEntryByPlaceholder(t, top, "New organization name")
	test.Type(entry, "Second Business")
	addBtn := findButtonByText(t, top, "Add")
	test.Tap(addBtn)

	// showError uses dialog.ShowError, which stacks a new overlay on top of
	// the still-open Manage dialog. This is the part of 6.6 that DOES work:
	// the failure is surfaced, not silently swallowed.
	errTop := a.win.Canvas().Overlays().Top()
	if errTop == nil || errTop == top {
		t.Fatal("expected an error dialog on top after Add failed on a read-only directory; failure appears to have been swallowed silently")
	}

	// Regression guard for D2, end to end through this exact UI flow (the
	// Registry half is TestAdd_FailedSaveLeavesRegistryUnchanged in
	// internal/org): the error dialog notwithstanding, a.orgs (in memory)
	// used to ALSO disagree with disk, because Registry.Add appended before
	// calling Save and did not roll back on failure — the switcher would go
	// on showing "Second Business" as real until the app restarted. Add now
	// rolls back on a failed Save, so the in-memory registry must match disk
	// here.
	os.Chmod(dir, 0o755)
	onDisk, err := org.Load(dir)
	if err != nil {
		t.Fatalf("reload registry from disk: %v", err)
	}
	if onDisk.Len() != 1 {
		t.Fatalf("test invariant broken: disk has %d organizations, want 1 before comparing to memory", onDisk.Len())
	}
	if a.orgs.Len() != onDisk.Len() {
		t.Errorf("after the failed Add, in-memory registry has %d organization(s) but disk has %d — "+
			"the app would show \"Second Business\" in the switcher even though it was never actually saved",
			a.orgs.Len(), onDisk.Len())
	}
	if _, ok := a.orgs.ByName("Second Business"); ok {
		t.Error("in-memory registry still contains \"Second Business\" after the failed Add")
	}
}
