package org

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAdd_DoesNotCreateDBFile is 6.1: Add registers a registry entry but
// never touches the .db file itself (store.Open, done by the caller, is
// what actually creates it). Confirms the premise behind the 6.1 finding:
// an org added and then removed without ever being opened leaves Remove
// pointing at a path that never existed.
func TestAdd_DoesNotCreateDBFile(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	o, err := r.Add("Never Opened")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	path := r.Path(o)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no .db file to exist yet at %q, stat error = %v", path, err)
	}
}

// TestRemove_NeverOpenedOrgReturnsPathToNonexistentFile is the other half of
// 6.1: Remove's return value (which the UI surfaces verbatim as "Its data
// file remains at: <path>") points at a file that was never created if the
// organization was never opened. The message is misleading in that specific
// case — there is nothing "remaining" there.
func TestRemove_NeverOpenedOrgReturnsPathToNonexistentFile(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := r.Add("Keep"); err != nil {
		t.Fatalf("Add(Keep): %v", err)
	}
	drop, err := r.Add("Never Opened")
	if err != nil {
		t.Fatalf("Add(Never Opened): %v", err)
	}

	path, err := r.Remove(drop.ID)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected %q to not exist (the org was never opened), stat error = %v", path, statErr)
	}
	t.Logf("Remove returned %q, which never existed — the UI's \"Its data file remains at: %s\" message is misleading here", path, path)
}

// TestTwoInstances_LastWriterWinsSilentlyDropsOtherInstancesAdd is 6.3:
// two in-memory Registry values loaded from the same directory (as would
// happen with two app instances/windows on the same data directory) do not
// coordinate. This is not something the package can fix without a locking
// protocol it doesn't have — the test documents the actual severity: a
// concurrent Add is silently lost, with no error surfaced to either
// instance.
func TestTwoInstances_LastWriterWinsSilentlyDropsOtherInstancesAdd(t *testing.T) {
	dir := t.TempDir()
	instance1, err := Load(dir)
	if err != nil {
		t.Fatalf("Load (instance1): %v", err)
	}
	if _, err := instance1.Add("Existing"); err != nil {
		t.Fatalf("Add(Existing): %v", err)
	}

	// instance2 opens the same directory after instance1's first write —
	// e.g. a second window opened moments later.
	instance2, err := Load(dir)
	if err != nil {
		t.Fatalf("Load (instance2): %v", err)
	}

	// instance1 adds a second organization (persisted to disk).
	added, err := instance1.Add("Added By Instance 1")
	if err != nil {
		t.Fatalf("Add(Added By Instance 1): %v", err)
	}

	// instance2, unaware of that write, renames "Existing" and saves — its
	// stale in-memory Organizations slice (which does not include "Added By
	// Instance 1") overwrites the file.
	existing, _ := instance2.ByName("Existing")
	if err := instance2.Rename(existing.ID, "Renamed By Instance 2"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	reloaded, err := Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, ok := reloaded.Get(added.ID); ok {
		t.Fatal("unexpected: instance 1's addition survived — the race did not reproduce as expected")
	}
	t.Logf("confirmed: instance 2's Save silently dropped instance 1's addition %q with no error to either side", added.Name)
	if _, ok := reloaded.ByName("Renamed By Instance 2"); !ok {
		t.Error("instance 2's own rename should at least have taken effect")
	}
}

// TestSave_OnReadOnlyDirectoryReturnsError is the registry half of 6.6: Save
// must surface a write failure as an error rather than silently discarding
// the change.
func TestSave_OnReadOnlyDirectoryReturnsError(t *testing.T) {
	skipUnlessChmodReadOnlyEnforced(t)
	parent := t.TempDir()
	dir := filepath.Join(parent, "data")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Seed one organization while the directory is still writable, then lock
	// it down, matching the realistic case: an existing data dir that
	// becomes read-only (e.g. a synced folder mid-sync, or restrictive
	// permissions after a restore).
	if _, err := r.Add("First"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	_, err = r.Add("Second")
	if err == nil {
		t.Fatal("expected Add (which calls Save) to fail loudly on a read-only directory, got nil error")
	}
	t.Logf("Save on a read-only directory correctly returned an error: %v", err)

	os.Chmod(dir, 0o755)
	reloaded, err := Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Len() != 1 {
		t.Errorf("Len = %d after a failed Add, want 1 (the failed add must not partially apply)", reloaded.Len())
	}
}

// TestAdd_FailedSaveLeavesRegistryUnchanged is the regression test for D2:
// Add used to append the new Organization to r.Organizations (and, for the
// first org, set r.ActiveID) BEFORE calling Save, and left both mutations in
// place when Save failed. The caller got a non-nil error, but the *Registry
// they were holding then silently disagreed with what was on disk. If the
// app kept running past the error dialog (which it does — see
// TestManageOrgsDialog_AddOnReadOnlyDirectorySurfacesError in internal/ui),
// the switcher showed an organization that a restart would make vanish. Add
// now snapshots and rolls back on a failed Save; this asserts that directly.
func TestAdd_FailedSaveLeavesRegistryUnchanged(t *testing.T) {
	skipUnlessChmodReadOnlyEnforced(t)
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := r.Add("First"); err != nil {
		t.Fatalf("Add(First): %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if _, err := r.Add("Second"); err == nil {
		t.Fatal("expected the second Add to fail on a read-only directory")
	}

	if r.Len() != 1 {
		t.Errorf("Registry.Len() = %d after a failed Add, want 1 — Add's append must be rolled back when Save fails", r.Len())
	}
	if _, ok := r.ByName("Second"); ok {
		t.Error("in-memory registry still contains \"Second\" even though Add returned an error and nothing was persisted to disk")
	}
}

// TestAdd_FailedSaveOnEmptyRegistryRollsBackActiveID covers the other half of
// Add's rollback: adding to a brand-new, empty registry also sets ActiveID
// (see Add's doc comment), which must revert too when Save fails — otherwise
// the registry would claim an organization is active that doesn't actually
// exist in r.Organizations. The orgs/ subdirectory is pre-created so Add's
// MkdirAll (which happens before the mutation this test targets) succeeds
// as a no-op even once the directory is locked down, and the failure this
// test wants comes from Save's os.WriteFile instead.
func TestAdd_FailedSaveOnEmptyRegistryRollsBackActiveID(t *testing.T) {
	skipUnlessChmodReadOnlyEnforced(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, orgsSubdir), 0o755); err != nil {
		t.Fatalf("mkdir orgs: %v", err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if _, err := r.Add("First"); err == nil {
		t.Fatal("expected Add to fail on a read-only directory")
	}

	if r.ActiveID != "" {
		t.Errorf("ActiveID = %q after a failed Add to an empty registry, want \"\" (rolled back)", r.ActiveID)
	}
	if r.Len() != 0 {
		t.Errorf("Registry.Len() = %d after a failed Add, want 0", r.Len())
	}
}

// TestRename_FailedSaveRollsBackName covers D2's Rename fix: a failed Save
// must not leave the running app showing a name that was never actually
// persisted.
func TestRename_FailedSaveRollsBackName(t *testing.T) {
	skipUnlessChmodReadOnlyEnforced(t)
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	first, err := r.Add("First")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if err := r.Rename(first.ID, "Renamed"); err == nil {
		t.Fatal("expected Rename (which calls Save) to fail on a read-only directory")
	}

	got, ok := r.Get(first.ID)
	if !ok {
		t.Fatal("organization vanished from the in-memory registry after a failed Rename")
	}
	if got.Name != "First" {
		t.Errorf("in-memory name = %q after a failed Rename, want the original %q restored", got.Name, "First")
	}
}

// TestRemove_FailedSaveRollsBackRemoval covers D2's Remove fix, the sharpest
// of the four: without a rollback, a failed Save would leave the
// organization gone from the running app (and ActiveID pointing elsewhere,
// if it was the active one) while its entry and database file are both
// still fully intact on disk.
func TestRemove_FailedSaveRollsBackRemoval(t *testing.T) {
	skipUnlessChmodReadOnlyEnforced(t)
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	first, err := r.Add("First")
	if err != nil {
		t.Fatalf("Add(First): %v", err)
	}
	if _, err := r.Add("Second"); err != nil {
		t.Fatalf("Add(Second): %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if _, err := r.Remove(first.ID); err == nil {
		t.Fatal("expected Remove (which calls Save) to fail on a read-only directory")
	}

	if r.Len() != 2 {
		t.Errorf("Registry.Len() = %d after a failed Remove, want 2 (removal must be rolled back)", r.Len())
	}
	if _, ok := r.Get(first.ID); !ok {
		t.Error("removed organization is missing from the in-memory registry even though Remove's Save failed")
	}
	if r.ActiveID != first.ID {
		t.Errorf("ActiveID = %q after a failed Remove of the active org, want it restored to %q", r.ActiveID, first.ID)
	}
}

// TestSetActive_FailedSaveRollsBackActiveID covers D2's SetActive fix.
func TestSetActive_FailedSaveRollsBackActiveID(t *testing.T) {
	skipUnlessChmodReadOnlyEnforced(t)
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	first, err := r.Add("First")
	if err != nil {
		t.Fatalf("Add(First): %v", err)
	}
	second, err := r.Add("Second")
	if err != nil {
		t.Fatalf("Add(Second): %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if err := r.SetActive(second.ID); err == nil {
		t.Fatal("expected SetActive (which calls Save) to fail on a read-only directory")
	}

	if r.ActiveID != first.ID {
		t.Errorf("ActiveID = %q after a failed SetActive, want it rolled back to %q", r.ActiveID, first.ID)
	}
}
