package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/model"
	"dock-invoice/internal/org"
	"dock-invoice/internal/store"
)

// ---- Phase 4: switching / isolation gaps ----------------------------------

// TestSwitchOrg_IsolatesMastersData extends TestSwitchOrg_IsolatesInvoices and
// TestSwitchOrg_IsolatesSettings (orgs_test.go) to customers and items, which
// neither of those cover (4.1 asks for settings + customer + item + invoice
// together).
func TestSwitchOrg_IsolatesMastersData(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]

	if _, err := a.db.SaveCustomer(model.Customer{Name: "First-Org Customer"}); err != nil {
		t.Fatalf("SaveCustomer: %v", err)
	}
	if _, err := a.db.SaveItem(model.Item{Description: "First-Org Item", HSNSAC: "998314"}); err != nil {
		t.Fatalf("SaveItem: %v", err)
	}

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	if custs, err := a.db.ListCustomers(); err != nil {
		t.Fatalf("ListCustomers: %v", err)
	} else if len(custs) != 0 {
		t.Errorf("second organization sees %d customer(s) from the first: %+v", len(custs), custs)
	}
	if items, err := a.db.ListItems(); err != nil {
		t.Fatalf("ListItems: %v", err)
	} else if len(items) != 0 {
		t.Errorf("second organization sees %d item(s) from the first: %+v", len(items), items)
	}

	if err := a.switchOrg(first.ID); err != nil {
		t.Fatalf("switchOrg back: %v", err)
	}
	custs, err := a.db.ListCustomers()
	if err != nil {
		t.Fatalf("ListCustomers: %v", err)
	}
	if len(custs) != 1 || custs[0].Name != "First-Org Customer" {
		t.Errorf("first organization's customers = %+v, want exactly First-Org Customer preserved", custs)
	}
	items, err := a.db.ListItems()
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 || items[0].Description != "First-Org Item" {
		t.Errorf("first organization's items = %+v, want exactly First-Org Item preserved", items)
	}
}

// TestSwitchOrg_NumberSeriesIsolation is 4.2, the sharpest form of leakage:
// an organization's invoice numbering must not continue another's counter.
func TestSwitchOrg_NumberSeriesIsolation(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]
	pattern := "AEX{FY}-{SEQ}"
	date := time.Now()

	// Advance org A's series to 2.
	n1, err := a.db.NextInvoiceNumber(pattern, date)
	if err != nil {
		t.Fatalf("NextInvoiceNumber (A, 1): %v", err)
	}
	seedInvoice(t, a.db, n1, "Customer A", date)
	n2, err := a.db.NextInvoiceNumber(pattern, date)
	if err != nil {
		t.Fatalf("NextInvoiceNumber (A, 2): %v", err)
	}
	seedInvoice(t, a.db, n2, "Customer A", date)
	if !strings.HasSuffix(n2, "-2") {
		t.Fatalf("expected org A's second number to end in -2, got %q", n2)
	}

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	bNum, err := a.db.NextInvoiceNumber(pattern, date)
	if err != nil {
		t.Fatalf("NextInvoiceNumber (B, 1): %v", err)
	}
	if !strings.HasSuffix(bNum, "-1") {
		t.Fatalf("org B's first number = %q, want it to start its own series at 1, not continue A's", bNum)
	}
	seedInvoice(t, a.db, bNum, "Customer B", date)

	if err := a.switchOrg(first.ID); err != nil {
		t.Fatalf("switchOrg back: %v", err)
	}
	aNum, err := a.db.NextInvoiceNumber(pattern, date)
	if err != nil {
		t.Fatalf("NextInvoiceNumber (A, 3): %v", err)
	}
	if !strings.HasSuffix(aNum, "-3") {
		t.Fatalf("org A's next number after switching back = %q, want it to continue from -2 (i.e. -3), not restart or continue B's series", aNum)
	}
}

// TestSettingsScreen_ReflectsActiveOrgAfterSwitch strengthens
// TestSwitchOrg_IsolatesSettings, which only checks a.settings in memory
// (4.3 also wants the rendered Settings screen widget tree to show the new
// org's values).
func TestSettingsScreen_ReflectsActiveOrgAfterSwitch(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]

	s := a.settings
	s.Company.Name = "First Business Pvt Ltd"
	if err := a.saveSettings(s); err != nil {
		t.Fatalf("saveSettings: %v", err)
	}

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	s2 := a.settings
	s2.Company.Name = "Second Business Inc"
	if err := a.saveSettings(s2); err != nil {
		t.Fatalf("saveSettings (second org): %v", err)
	}
	// buildSettingsContainer rebuilds the screen fresh each time it's called
	// (see App.build), reading a.settings.
	container := a.buildSettingsContainer()
	nameEntry := findEntryWithText(t, container, "Second Business Inc")
	if nameEntry == nil {
		t.Fatal("Settings screen does not show the second organization's company name")
	}

	if err := a.switchOrg(first.ID); err != nil {
		t.Fatalf("switchOrg back: %v", err)
	}
	container = a.buildSettingsContainer()
	if findEntryWithText(t, container, "First Business Pvt Ltd") == nil {
		t.Fatal("Settings screen does not show the first organization's company name after switching back")
	}
	if findEntryWithText(t, container, "Second Business Inc") != nil {
		t.Fatal("Settings screen still shows the second organization's company name after switching back to the first")
	}
}

func findEntryWithText(t *testing.T, root any, text string) *widget.Entry {
	t.Helper()
	var found *widget.Entry
	walk(root, func(o any) {
		if found != nil {
			return
		}
		if e, ok := o.(*widget.Entry); ok && e.Text == text {
			found = e
		}
	})
	return found
}

// TestSwitchOrg_EditorTabResetToPlaceholder covers the second half of 4.4:
// after switchOrg the Editor tab must be back to its placeholder, not still
// showing the previous organization's (now torn-down) draft editor.
func TestSwitchOrg_EditorTabResetToPlaceholder(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	second := reg.List()[1]

	state := NewEditorState(a.settings)
	state.Number = "AEX2425-1"
	a.openEditor(state)
	if a.tabs.Selected() != a.editorTab {
		t.Fatal("openEditor should have selected the Editor tab")
	}

	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}

	if a.editorOpen {
		t.Error("editorOpen still true after switchOrg rebuilt the tabs")
	}
	// The placeholder content has no Save/Cancel buttons and no number entry
	// — walk it and confirm no widget.Entry exists (the placeholder is
	// emptyState(), pure labels/icons).
	var entryCount int
	walk(a.editorTab.Content, func(o any) {
		if _, ok := o.(*widget.Entry); ok {
			entryCount++
		}
	})
	if entryCount != 0 {
		t.Errorf("Editor tab still holds %d entry widget(s) after switching organizations; want the placeholder", entryCount)
	}
}

// TestSwitchOrg_ClosesOldHandleNotNewOne covers 4.5: after a switch, the
// previous *store.DB must be closed, and it must specifically be the OLD
// handle that got closed, not the new one by accident.
func TestSwitchOrg_ClosesOldHandleNotNewOne(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	second := reg.List()[1]

	oldDB := a.db
	if err := a.switchOrg(second.ID); err != nil {
		t.Fatalf("switchOrg: %v", err)
	}
	newDB := a.db

	if oldDB == newDB {
		t.Fatal("a.db was not replaced by switchOrg")
	}
	if _, err := oldDB.ListInvoices(); err == nil {
		t.Error("old database handle still usable after switchOrg; want it closed")
	}
	if _, err := newDB.ListInvoices(); err != nil {
		t.Errorf("new database handle is not usable after switchOrg: %v", err)
	}
}

// TestSwitchOrg_FailedSwitchIsAtomic covers 4.6: switchOrg's comment
// explicitly promises the app stays exactly where it was if opening the
// target fails. Force store.Open to fail by pointing the target org's dbFile
// at a path that is an existing directory (sql.Open is lazy; the failure
// surfaces on the first Exec inside store.Open, e.g. PRAGMA/schema apply).
func TestSwitchOrg_FailedSwitchIsAtomic(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	second := reg.List()[1]

	// Replace the second organization's database FILE with a directory of
	// the same name so store.Open cannot open it as a sqlite file.
	badPath := reg.Path(second)
	if err := os.MkdirAll(badPath, 0o755); err != nil {
		t.Fatalf("seed directory at %s: %v", badPath, err)
	}

	activeBefore := a.orgs.ActiveID
	dbPathBefore := a.dbPath
	screenBefore := a.invoicesScreen

	err := a.switchOrg(second.ID)
	if err == nil {
		t.Fatal("expected switchOrg to fail when the target path is a directory")
	}

	if a.orgs.ActiveID != activeBefore {
		t.Errorf("ActiveID changed to %q after a failed switch, want it unchanged (%q)", a.orgs.ActiveID, activeBefore)
	}
	if a.dbPath != dbPathBefore {
		t.Errorf("dbPath changed to %q after a failed switch, want it unchanged (%q)", a.dbPath, dbPathBefore)
	}
	if a.invoicesScreen != screenBefore {
		t.Error("screens were rebuilt despite the failed switch")
	}
	if _, err := a.db.ListInvoices(); err != nil {
		t.Errorf("original database is no longer queryable after a failed switch: %v", err)
	}

	// And the registry on disk must not have recorded the failed target as
	// active either (SetActive is only called after store.Open succeeds).
	if a.orgs.ActiveID == second.ID {
		t.Error("registry ActiveID was updated despite the failed open")
	}
}

// ---- Phase 5: UI behaviour --------------------------------------------------

// TestManageDialog_ReflectsAddAndRemove covers the add/remove half of 5.1:
// the dialog is rebuilt from the registry after every mutation, so it must
// never show a stale list. Rename is covered by TestRename_ReflectedInWindowTitle.
func TestManageDialog_ReflectsAddAndRemove(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	activeBefore, _ := reg.Active()

	third, err := reg.Add("Third Business")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	a.showManageOrgs()
	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("manage dialog did not open")
	}
	if got := len(buttonsByText(top, "Open")); got != 3 {
		t.Fatalf("manage dialog shows %d rows after Add, want 3", got)
	}
	// Adding must not move the app off the organization still open.
	if a.orgs.ActiveID != activeBefore.ID {
		t.Errorf("ActiveID = %q after Add, want it to stay %q", a.orgs.ActiveID, activeBefore.ID)
	}

	if _, err := reg.Remove(third.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	a.showManageOrgs()
	top = a.win.Canvas().Overlays().Top()
	if got := len(buttonsByText(top, "Open")); got != 2 {
		t.Fatalf("manage dialog shows %d rows after Remove, want 2", got)
	}
	var names []string
	walk(top, func(o any) {
		if l, ok := o.(*widget.Label); ok {
			names = append(names, l.Text)
		}
	})
	for _, n := range names {
		if n == "Third Business" {
			t.Error("manage dialog still lists the removed organization")
		}
	}
}

// TestConfirmOrgSwitch_DeclineDoesNotSwitch is 5.3: declining the
// discard-draft confirmation must leave the app on the organization that is
// still open, with the unsaved draft intact.
func TestConfirmOrgSwitch_DeclineDoesNotSwitch(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]

	state := NewEditorState(a.settings)
	state.Number = "AEX2425-1"
	a.openEditor(state)
	if !a.editorOpen {
		t.Fatal("test setup: expected editorOpen after openEditor")
	}

	dbBefore := a.db
	a.confirmOrgSwitch(second)

	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected a confirmation dialog to be showing")
	}
	no := findButtonByText(t, top, "No")
	test.Tap(no)

	if a.win.Canvas().Overlays().Top() != nil {
		t.Error("dialog still showing after declining")
	}
	if a.orgs.ActiveID != first.ID {
		t.Errorf("ActiveID = %q after declining the switch, want it to stay %q", a.orgs.ActiveID, first.ID)
	}
	if a.db != dbBefore {
		t.Error("switchOrg actually ran despite the switch being declined")
	}
	if !a.editorOpen {
		t.Error("editorOpen was cleared even though the switch was declined and the draft is still there")
	}
}

// TestConfirmOrgSwitch_ConfirmProceeds is the accept half of the same flow,
// confirming the dialog path actually performs the switch (not just that
// decline reverts it).
func TestConfirmOrgSwitch_ConfirmProceeds(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	second := reg.List()[1]

	state := NewEditorState(a.settings)
	state.Number = "AEX2425-1"
	a.openEditor(state)

	a.confirmOrgSwitch(second)
	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected a confirmation dialog to be showing")
	}
	yes := findButtonByText(t, top, "Yes")
	test.Tap(yes)

	if a.orgs.ActiveID != second.ID {
		t.Errorf("ActiveID = %q after confirming the switch, want %q", a.orgs.ActiveID, second.ID)
	}
	if a.editorOpen {
		t.Error("editorOpen still true after a confirmed switch tore down the draft")
	}
}

// TestConfirmOrgSwitch_NoDraftSwitchesWithoutPrompting covers the "switching
// without one does not [prompt]" half of 5.4.
func TestConfirmOrgSwitch_NoDraftSwitchesWithoutPrompting(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	second := reg.List()[1]
	if a.editorOpen {
		t.Fatal("test setup: expected no draft open")
	}

	a.confirmOrgSwitch(second)

	if top := a.win.Canvas().Overlays().Top(); top != nil {
		t.Error("confirmOrgSwitch prompted even though no draft was open")
	}
	if a.orgs.ActiveID != second.ID {
		t.Errorf("ActiveID = %q, want the switch to have proceeded immediately to %q", a.orgs.ActiveID, second.ID)
	}
}

// TestEditorOpen_ClearedBySwitch_SecondSwitchDoesNotWarn is 5.5: after a
// switch clears editorOpen, a second switch must not still think a draft is
// open (it isn't — the Editor tab was already rebuilt to the placeholder).
func TestEditorOpen_ClearedBySwitch_SecondSwitchDoesNotWarn(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]

	state := NewEditorState(a.settings)
	state.Number = "AEX2425-1"
	a.openEditor(state)

	// First switch: confirm through the dialog since a draft is open.
	a.confirmOrgSwitch(second)
	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected a confirmation dialog on the first switch")
	}
	test.Tap(findButtonByText(t, top, "Yes"))
	if a.orgs.ActiveID != second.ID {
		t.Fatalf("first switch did not take effect, ActiveID = %q", a.orgs.ActiveID)
	}

	// Second switch, back to the first org: editorOpen should already be
	// false, so this must proceed without any dialog.
	a.confirmOrgSwitch(first)
	if top := a.win.Canvas().Overlays().Top(); top != nil {
		t.Error("second switch warned about a draft that no longer exists")
	}
	if a.orgs.ActiveID != first.ID {
		t.Errorf("ActiveID = %q after the second switch, want %q", a.orgs.ActiveID, first.ID)
	}
}

// ---- Manage-organizations dialog flows (5.6, 5.7, 5.8, 5.9) ---------------

// manageDialogBody returns the currently-showing "Organizations" dialog's
// content overlay, failing the test if none is showing.
func manageDialogBody(t *testing.T, a *App) any {
	t.Helper()
	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected the Organizations dialog to be showing")
	}
	return top
}

// TestManageOrgsDialog_AddFlow is 5.6: adding through the dialog refreshes
// the switcher and offers to open the new org; declining leaves the user on
// their current org.
func TestManageOrgsDialog_AddFlow(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	activeBefore, _ := reg.Active()

	a.showManageOrgs()
	body := manageDialogBody(t, a)

	entry := findEntryByPlaceholder(t, body, "New organization name")
	test.Type(entry, "Third Business")
	addBtn := findButtonByText(t, body, "Add")
	test.Tap(addBtn)

	if reg.Len() != 3 {
		t.Fatalf("registry Len = %d after Add, want 3", reg.Len())
	}
	if got := len(buttonsByText(body, "Open")); got != 3 {
		t.Errorf("manage dialog shows %d rows after adding from the dialog, want 3", got)
	}

	// A follow-up "Open Third Business?" confirm should now be on top.
	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected an 'Open new org?' confirmation after Add")
	}
	no := findButtonByText(t, top, "No")
	test.Tap(no)

	if a.orgs.ActiveID != activeBefore.ID {
		t.Errorf("ActiveID = %q after declining to open the new org, want it to stay %q", a.orgs.ActiveID, activeBefore.ID)
	}
}

// TestManageOrgsDialog_RemoveCurrentOrg is 5.7: removing the org that is
// currently open must follow the registry to the fallback and rebuild.
func TestManageOrgsDialog_RemoveCurrentOrg(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]
	if a.orgs.ActiveID != first.ID {
		t.Fatalf("test setup: expected %q active", first.ID)
	}
	screenBefore := a.invoicesScreen

	// orgRow's delete button's OnTapped is exactly `a.promptRemoveOrg(o,
	// refresh)` (see orgs.go); calling it directly exercises the same
	// production code path without needing a pixel-perfect widget-tree walk
	// to disambiguate two identical-looking icon-only delete buttons in the
	// Manage dialog (one per row).
	refresh := func() {}
	a.promptRemoveOrg(first, refresh)

	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected a remove-confirmation dialog")
	}
	test.Tap(findButtonByText(t, top, "Yes"))

	if a.orgs.ActiveID != second.ID {
		t.Fatalf("ActiveID = %q after removing the open org, want fallback %q", a.orgs.ActiveID, second.ID)
	}
	if a.invoicesScreen == screenBefore {
		t.Error("screens were not rebuilt after removing the currently-open organization")
	}
	if a.dbPath != reg.Path(second) {
		t.Errorf("dbPath = %q, want %q", a.dbPath, reg.Path(second))
	}
	if _, err := a.db.ListInvoices(); err != nil {
		t.Errorf("new active database is not queryable after remove-and-reopen: %v", err)
	}

	// A final "Removed" info dialog should report the leftover path.
	top2 := a.win.Canvas().Overlays().Top()
	if top2 == nil {
		t.Fatal("expected an info dialog reporting where the removed org's data file remains")
	}
}

// TestManageOrgsDialog_RemoveNonOpenOrg is 5.8: removing an org that is NOT
// open must leave the currently-open org and its DB handle untouched.
func TestManageOrgsDialog_RemoveNonOpenOrg(t *testing.T) {
	a, reg := newMultiOrgTestApp(t)
	orgs := reg.List()
	first, second := orgs[0], orgs[1]
	if a.orgs.ActiveID != first.ID {
		t.Fatalf("test setup: expected %q active", first.ID)
	}
	dbBefore := a.db
	screenBefore := a.invoicesScreen

	refresh := func() {}
	a.promptRemoveOrg(second, refresh)
	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected a remove-confirmation dialog")
	}
	test.Tap(findButtonByText(t, top, "Yes"))

	if a.orgs.ActiveID != first.ID {
		t.Errorf("ActiveID changed to %q after removing a non-open org, want it to stay %q", a.orgs.ActiveID, first.ID)
	}
	if a.db != dbBefore {
		t.Error("the open org's DB handle was replaced after removing a different, non-open org")
	}
	if a.invoicesScreen != screenBefore {
		t.Error("screens were rebuilt after removing a non-open org (unnecessary work, and a UI flicker)")
	}
	if _, err := a.db.ListInvoices(); err != nil {
		t.Errorf("open org's database unusable after an unrelated remove: %v", err)
	}
	if _, ok := reg.Get(second.ID); ok {
		t.Error("removed organization is still present in the registry")
	}
}

// TestManageOrgsDialog_RemoveLastOrgBlocked is 5.9.
func TestManageOrgsDialog_RemoveLastOrgBlocked(t *testing.T) {
	dir := t.TempDir()
	regOnly := mustSingleOrgRegistry(t, dir, "Solo Business")
	path, _ := regOnly.ActivePath()
	db := mustOpenDB(t, path)

	a, err := NewWorkspaceAppWithFyneApp(test.NewTempApp(t), &Workspace{DB: db, Path: path, Orgs: regOnly})
	if err != nil {
		t.Fatalf("NewWorkspaceAppWithFyneApp: %v", err)
	}
	t.Cleanup(func() { a.Close() })

	only, _ := regOnly.Active()
	refresh := func() {}
	a.promptRemoveOrg(only, refresh)

	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("expected an error dialog blocking removal of the last organization")
	}
	// promptRemoveOrg's Len()<=1 guard fires *before* any confirm dialog —
	// showError uses dialog.ShowError, whose dismiss button is "OK".
	ok := findButtonByText(t, top, "OK")
	test.Tap(ok)

	if regOnly.Len() != 1 {
		t.Errorf("registry Len = %d, want 1 — the last organization must not be removable", regOnly.Len())
	}
}

func mustSingleOrgRegistry(t *testing.T, dir, name string) *org.Registry {
	t.Helper()
	r, err := org.Load(dir)
	if err != nil {
		t.Fatalf("org.Load: %v", err)
	}
	if _, err := r.Add(name); err != nil {
		t.Fatalf("Add: %v", err)
	}
	return r
}

func mustOpenDB(t *testing.T, path string) *store.DB {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
