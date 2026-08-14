package org

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_MissingFileYieldsEmptyRegistry(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.Len() != 0 {
		t.Errorf("Len = %d, want 0 for a fresh directory", r.Len())
	}
	if _, ok := r.Active(); ok {
		t.Error("expected no active organization in an empty registry")
	}
}

func TestAddAndPersist(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	acme, err := r.Add("Acme Electricals")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if acme.ID != "acme-electricals" {
		t.Errorf("ID = %q, want %q", acme.ID, "acme-electricals")
	}
	if acme.DBFile != "orgs/acme-electricals.db" {
		t.Errorf("DBFile = %q, want %q", acme.DBFile, "orgs/acme-electricals.db")
	}
	// The first organization added has to become active, or the app would
	// start with nothing to open.
	if r.ActiveID != acme.ID {
		t.Errorf("ActiveID = %q, want the first organization %q", r.ActiveID, acme.ID)
	}

	second, err := r.Add("Second Business")
	if err != nil {
		t.Fatalf("Add second: %v", err)
	}
	// Adding must not steal focus from the organization being worked in.
	if r.ActiveID != acme.ID {
		t.Errorf("ActiveID = %q after adding a second org, want it to stay %q", r.ActiveID, acme.ID)
	}

	reloaded, err := Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Len() != 2 {
		t.Fatalf("reloaded Len = %d, want 2", reloaded.Len())
	}
	if reloaded.ActiveID != acme.ID {
		t.Errorf("reloaded ActiveID = %q, want %q", reloaded.ActiveID, acme.ID)
	}
	if got, ok := reloaded.Get(second.ID); !ok || got.Name != "Second Business" {
		t.Errorf("reloaded Get(%q) = %+v, %v", second.ID, got, ok)
	}
}

func TestAdd_RejectsDuplicateNameCaseInsensitively(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := r.Add("Acme"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := r.Add("  acme  "); err == nil {
		t.Fatal("expected duplicate name to be rejected")
	}
	if r.Len() != 1 {
		t.Errorf("Len = %d, want 1 after the rejected add", r.Len())
	}
}

func TestAdd_RejectsBlankName(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := r.Add("   "); err == nil {
		t.Fatal("expected a blank name to be rejected")
	}
}

// TestAdd_UniquifiesCollidingSlugs covers two different display names that
// slugify to the same ID — they must still get distinct database files.
func TestAdd_UniquifiesCollidingSlugs(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a, err := r.Add("Acme & Co")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	b, err := r.Add("Acme  Co.")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if a.ID == b.ID {
		t.Fatalf("both organizations got ID %q", a.ID)
	}
	if a.DBFile == b.DBFile {
		t.Fatalf("both organizations share database file %q", a.DBFile)
	}
}

// TestSlugify_NonLatinNameStillGetsAnID pins the fallback: a name with no
// ASCII alphanumerics must not produce an empty ID (and therefore an
// unopenable ".db" file).
func TestSlugify_NonLatinNameStillGetsAnID(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	o, err := r.Add("राजेंद्र इलेक्ट्रिकल्स")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if o.ID == "" {
		t.Fatal("ID is empty")
	}
	if strings.HasPrefix(filepath.Base(o.DBFile), ".") {
		t.Errorf("DBFile %q has no basename before the extension", o.DBFile)
	}
}

func TestAdoptLegacy_KeepsExistingDatabaseInPlace(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	o, err := r.AdoptLegacy("Rajender Electricals", "invoicer.db")
	if err != nil {
		t.Fatalf("AdoptLegacy: %v", err)
	}
	if o.DBFile != "invoicer.db" {
		t.Errorf("DBFile = %q, want the legacy path %q untouched", o.DBFile, "invoicer.db")
	}
	if want := filepath.Join(dir, "invoicer.db"); r.Path(o) != want {
		t.Errorf("Path = %q, want %q", r.Path(o), want)
	}
	if r.ActiveID != o.ID {
		t.Errorf("ActiveID = %q, want %q", r.ActiveID, o.ID)
	}
}

func TestAdoptLegacy_NoOpWhenRegistryAlreadyPopulated(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	first, err := r.Add("Acme")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := r.AdoptLegacy("Should Not Appear", "invoicer.db")
	if err != nil {
		t.Fatalf("AdoptLegacy: %v", err)
	}
	if got.ID != first.ID {
		t.Errorf("AdoptLegacy returned %q, want the existing active org %q", got.ID, first.ID)
	}
	if r.Len() != 1 {
		t.Errorf("Len = %d, want 1 — AdoptLegacy must not add to a populated registry", r.Len())
	}
}

func TestSetActive(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := r.Add("First"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	second, err := r.Add("Second")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := r.SetActive(second.ID); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if err := r.SetActive("nope"); err == nil {
		t.Fatal("expected SetActive on an unknown id to fail")
	}

	// The active choice must survive a restart.
	reloaded, err := Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.ActiveID != second.ID {
		t.Errorf("reloaded ActiveID = %q, want %q", reloaded.ActiveID, second.ID)
	}
}

func TestRename(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	o, err := r.Add("Old Name")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	other, err := r.Add("Other")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := r.Rename(o.ID, "New Name"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	got, _ := r.Get(o.ID)
	if got.Name != "New Name" {
		t.Errorf("Name = %q, want %q", got.Name, "New Name")
	}
	// Renaming must not disturb the ID or database file, or the rename
	// would orphan the data.
	if got.DBFile != o.DBFile {
		t.Errorf("DBFile changed on rename: %q -> %q", o.DBFile, got.DBFile)
	}

	if err := r.Rename(o.ID, "Other"); err == nil {
		t.Fatal("expected renaming onto an existing name to fail")
	}
	if err := r.Rename(other.ID, "Other"); err != nil {
		t.Fatalf("renaming an organization to its own name should be allowed: %v", err)
	}
	if err := r.Rename("missing", "X"); err == nil {
		t.Fatal("expected Rename on an unknown id to fail")
	}
}

func TestRemove_ReturnsFileAndKeepsItOnDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	keep, err := r.Add("Keep")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	drop, err := r.Add("Drop")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Simulate the database file the app would have created.
	dropPath := r.Path(drop)
	if err := os.WriteFile(dropPath, []byte("not really sqlite"), 0o644); err != nil {
		t.Fatalf("seed db file: %v", err)
	}

	path, err := r.Remove(drop.ID)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if path != dropPath {
		t.Errorf("Remove returned %q, want %q", path, dropPath)
	}
	// Removing from the list must never destroy retained tax records.
	if _, err := os.Stat(dropPath); err != nil {
		t.Errorf("database file was deleted by Remove: %v", err)
	}
	if r.Len() != 1 {
		t.Errorf("Len = %d, want 1", r.Len())
	}
	if r.ActiveID != keep.ID {
		t.Errorf("ActiveID = %q, want %q", r.ActiveID, keep.ID)
	}
}

func TestRemove_ActiveFallsBackToFirstRemaining(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	first, _ := r.Add("First")
	second, _ := r.Add("Second")
	if err := r.SetActive(second.ID); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	if _, err := r.Remove(second.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if r.ActiveID != first.ID {
		t.Errorf("ActiveID = %q, want %q after removing the active organization", r.ActiveID, first.ID)
	}
}

func TestRemove_RefusesLastOrganization(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	only, err := r.Add("Only")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := r.Remove(only.ID); !errors.Is(err, ErrLastOrganization) {
		t.Fatalf("Remove = %v, want ErrLastOrganization", err)
	}
	if _, err := r.Remove("unknown"); err == nil {
		t.Fatal("expected Remove on an unknown id to fail")
	}
}

// TestLoad_RejectsEscapingDBFile is the security guard: organizations.json is
// a plain file in a user-writable directory, so a doctored DBFile must not be
// able to steer the app at an arbitrary path.
func TestLoad_RejectsEscapingDBFile(t *testing.T) {
	for _, dbFile := range []string{"../elsewhere.db", "/etc/passwd", ""} {
		t.Run(dbFile, func(t *testing.T) {
			dir := t.TempDir()
			body := `{"organizations":[{"id":"x","name":"X","dbFile":"` + dbFile + `"}],"activeId":"x"}`
			if err := os.WriteFile(filepath.Join(dir, RegistryFileName), []byte(body), 0o644); err != nil {
				t.Fatalf("write registry: %v", err)
			}
			if _, err := Load(dir); err == nil {
				t.Fatalf("expected Load to reject dbFile %q", dbFile)
			}
		})
	}
}

func TestLoad_RejectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	body := `{"organizations":[{"id":"x","name":"A","dbFile":"a.db"},{"id":"x","name":"B","dbFile":"b.db"}],"activeId":"x"}`
	if err := os.WriteFile(filepath.Join(dir, RegistryFileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected Load to reject duplicate ids")
	}
}

// TestLoad_DanglingActiveIDFallsBackToFirst keeps the app startable when the
// active organization has gone missing from the list.
func TestLoad_DanglingActiveIDFallsBackToFirst(t *testing.T) {
	dir := t.TempDir()
	body := `{"organizations":[{"id":"a","name":"A","dbFile":"a.db"}],"activeId":"ghost"}`
	if err := os.WriteFile(filepath.Join(dir, RegistryFileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.ActiveID != "a" {
		t.Errorf("ActiveID = %q, want the fallback %q", r.ActiveID, "a")
	}
}

func TestLoad_RequiresDirectory(t *testing.T) {
	if _, err := Load(""); err == nil {
		t.Fatal("expected Load(\"\") to fail")
	}
}

func TestNamesAndListAreCopies(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := r.Add("Alpha"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := r.Add("Beta"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	names := r.Names()
	if len(names) != 2 || names[0] != "Alpha" || names[1] != "Beta" {
		t.Fatalf("Names = %v, want [Alpha Beta]", names)
	}

	list := r.List()
	list[0].Name = "Mutated"
	if got, _ := r.Get(list[0].ID); got.Name != "Alpha" {
		t.Error("List returned a view into the registry; callers must get a copy")
	}
}
