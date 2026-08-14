package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"dock-invoice/internal/org"
	"dock-invoice/internal/store"
)

// TestVisual_OrgBarCostsNoLayout renders both shells — single-database (no
// button) and multi-organization (the Organizations button laid over the tab
// strip) — and pins the property the button was moved onto that strip for: the
// organization UI must not spend any of the window on itself. The tab area has
// to come out exactly the same size in both modes.
//
// It also writes PNGs (like visual_test.go's TestVisual_Screens) for human
// review, in both modes — placement on the strip is a visual question, and the
// screenshot is the check for it.
func TestVisual_OrgBarCostsNoLayout(t *testing.T) {
	const w, h = 1280, 860

	// -- single-database mode ---------------------------------------------
	singleApp := test.NewTempApp(t)
	singleDB, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { singleDB.Close() })
	a1, err := NewAppWithFyneApp(singleApp, singleDB, "")
	if err != nil {
		t.Fatalf("NewAppWithFyneApp: %v", err)
	}
	if orgsButton(a1) != nil {
		t.Fatal("test invariant broken: single-database mode should have no org bar")
	}

	win1 := test.NewTempWindow(t, a1.win.Content())
	win1.Resize(fyne.NewSize(w, h))
	win1.Show()
	shoot(t, "shell-single-db-mode", fyne.NewSize(w, h), a1.win.Content())

	tabsPos1 := a1.tabs.Position()
	tabsSize1 := a1.tabs.Size()

	// -- multi-organization mode ------------------------------------------
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
		t.Fatal("no active organization")
	}
	multiDB, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { multiDB.Close() })

	multiApp := test.NewTempApp(t)
	a2, err := NewWorkspaceAppWithFyneApp(multiApp, &Workspace{DB: multiDB, Path: path, Orgs: reg})
	if err != nil {
		t.Fatalf("NewWorkspaceAppWithFyneApp: %v", err)
	}
	if orgsButton(a2) == nil {
		t.Fatal("test invariant broken: multi-organization mode should show an org bar")
	}

	win2 := test.NewTempWindow(t, a2.win.Content())
	win2.Resize(fyne.NewSize(w, h))
	win2.Show()
	shoot(t, "shell-multi-org-mode", fyne.NewSize(w, h), a2.win.Content())

	tabsPos2 := a2.tabs.Position()
	tabsSize2 := a2.tabs.Size()

	// Position is relative to each mode's own parent — the bare tabs in one
	// case, the Stack in the other — so only the size is comparable, and it
	// is the size that carries the claim.
	if tabsSize2 != tabsSize1 {
		t.Errorf("multi-org tab area = %v, want it identical to single-database mode's %v — the organization button must not shrink the working area", tabsSize2, tabsSize1)
	}
	if tabsPos1.Y == 0 && tabsPos2.Y != 0 {
		t.Errorf("multi-org tabs origin Y = %v, want 0 — the button overlays the strip rather than pushing it down", tabsPos2.Y)
	}
}
