package org

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdd_ThreeWaySlugCollision strengthens the existing two-way collision
// coverage in TestAdd_UniquifiesCollidingSlugs (2.4): the plan calls for a
// third colliding name and an explicit -2/-3 suffix pattern, plus a global
// "no two organizations ever share a DBFile" check across the whole registry
// rather than just the pair being compared.
func TestAdd_ThreeWaySlugCollision(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a, err := r.Add("Acme Ltd")
	if err != nil {
		t.Fatalf("Add(Acme Ltd): %v", err)
	}
	b, err := r.Add("acme-ltd")
	if err != nil {
		t.Fatalf("Add(acme-ltd): %v", err)
	}
	c, err := r.Add("Acme, Ltd.")
	if err != nil {
		t.Fatalf("Add(Acme, Ltd.): %v", err)
	}

	if a.ID != "acme-ltd" {
		t.Errorf("first ID = %q, want %q", a.ID, "acme-ltd")
	}
	if b.ID != "acme-ltd-2" {
		t.Errorf("second ID = %q, want %q", b.ID, "acme-ltd-2")
	}
	if c.ID != "acme-ltd-3" {
		t.Errorf("third ID = %q, want %q", c.ID, "acme-ltd-3")
	}

	seen := map[string]string{} // DBFile -> ID
	for _, o := range r.List() {
		if owner, dup := seen[o.DBFile]; dup {
			t.Fatalf("DBFile %q shared by %q and %q", o.DBFile, owner, o.ID)
		}
		seen[o.DBFile] = o.ID
	}
}

// TestSlugify_LongNameTruncatedWithoutTrailingDash covers the >40-char name
// edge case from 2.5, which nothing in registry_test.go exercises.
func TestSlugify_LongNameTruncatedWithoutTrailingDash(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// 60 hyphen-separated words -> after slugify, well past 40 chars, and the
	// cut point (character 40) lands mid-run-of-dashes for this input, which
	// is exactly what exercises the TrimRight("-").
	name := strings.Repeat("word ", 20) // slugifies to "word-word-word-...-word" (99 chars)
	o, err := r.Add(name)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(o.ID) > 40 {
		t.Errorf("ID %q is %d chars, want <= 40", o.ID, len(o.ID))
	}
	if strings.HasSuffix(o.ID, "-") {
		t.Errorf("ID %q has a trailing dash after truncation", o.ID)
	}
}

// TestSlugify_PunctuationOnlyNameFallsBackToOrg covers a name of only
// punctuation (2.5): slugify must not yield an empty ID.
func TestSlugify_PunctuationOnlyNameFallsBackToOrg(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	o, err := r.Add("!!!")
	if err != nil {
		t.Fatalf("Add(\"!!!\"): %v", err)
	}
	if o.ID != "org" {
		t.Errorf("ID = %q, want fallback %q", o.ID, "org")
	}

	// A second punctuation-only name must not collide silently with the
	// first — it should be uniquified the same way any other slug collision
	// is.
	o2, err := r.Add("???")
	if err != nil {
		t.Fatalf("Add(\"???\"): %v", err)
	}
	if o2.ID != "org-2" {
		t.Errorf("second punctuation-only ID = %q, want %q", o2.ID, "org-2")
	}
}

// TestRename_RejectsBlankName is the Rename counterpart of
// TestAdd_RejectsBlankName (2.6) — nothing in registry_test.go currently
// exercises this path, only Add's.
func TestRename_RejectsBlankName(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	o, err := r.Add("Original")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Rename(o.ID, "   "); err == nil {
		t.Fatal("expected Rename to reject a blank name")
	}
	got, _ := r.Get(o.ID)
	if got.Name != "Original" {
		t.Errorf("Name = %q after a rejected blank rename, want it unchanged", got.Name)
	}
}

// TestRemove_RefusesLastOrganization_RegistryUnchangedOnDisk strengthens
// TestRemove_RefusesLastOrganization (2.8): the plan explicitly wants the
// on-disk file verified unchanged, not just the returned error.
func TestRemove_RefusesLastOrganization_RegistryUnchangedOnDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	only, err := r.Add("Only")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, RegistryFileName))
	if err != nil {
		t.Fatalf("read registry before: %v", err)
	}

	if _, err := r.Remove(only.ID); err == nil {
		t.Fatal("expected Remove to refuse the last organization")
	}

	after, err := os.ReadFile(filepath.Join(dir, RegistryFileName))
	if err != nil {
		t.Fatalf("read registry after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("registry file changed after a refused Remove:\nbefore: %s\nafter:  %s", before, after)
	}
	if r.Len() != 1 {
		t.Errorf("in-memory Len = %d, want 1", r.Len())
	}
}

// TestLoad_RejectsEmptyID covers the "empty id" case from 2.10, which
// TestLoad_RejectsEscapingDBFile (dbFile cases) and TestLoad_RejectsDuplicateIDs
// (duplicate ids) do not.
func TestLoad_RejectsEmptyID(t *testing.T) {
	dir := t.TempDir()
	body := `{"organizations":[{"id":"","name":"X","dbFile":"x.db"}],"activeId":""}`
	if err := os.WriteFile(filepath.Join(dir, RegistryFileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected Load to reject an empty organization id")
	}
}

// TestLoad_RejectsMalformedJSON covers the "malformed JSON" case from 2.10.
func TestLoad_RejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, RegistryFileName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected Load to reject malformed JSON")
	}
}

// TestSave_LeavesNoTempFileBehind covers 2.12's atomicity claim: after a
// successful Save, no organizations.json.tmp should remain.
func TestSave_LeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := r.Add("Acme"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, RegistryFileName+".tmp")); !os.IsNotExist(err) {
		t.Errorf("expected no .tmp file after Save, stat error = %v", err)
	}
}

// TestLoad_StaleTempFileOnlyYieldsEmptyRegistry covers the other half of
// 2.12: a directory holding only a stale .tmp (organizations.json itself
// missing, e.g. the process died between WriteFile and Rename) must load as
// a fresh empty registry, not fail to parse.
func TestLoad_StaleTempFileOnlyYieldsEmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, RegistryFileName+".tmp"), []byte(`{"organizations":[{"id":"x","name":"X","dbFile":"x.db"}],"activeId":"x"}`), 0o644); err != nil {
		t.Fatalf("write stale tmp: %v", err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.Len() != 0 {
		t.Errorf("Len = %d, want 0 — a stale .tmp file must not be read as the registry", r.Len())
	}
}

// TestRegistry_Relocatable proves the stated reason DBFile is stored relative
// to the registry directory (2.14): build a registry in one directory, move
// the whole directory elsewhere, and confirm Path() resolves under the new
// location rather than the old one.
func TestRegistry_Relocatable(t *testing.T) {
	parent := t.TempDir()
	dirA := filepath.Join(parent, "A")
	if err := os.Mkdir(dirA, 0o755); err != nil {
		t.Fatalf("mkdir A: %v", err)
	}
	r, err := Load(dirA)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	o, err := r.Add("Acme")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	dirB := filepath.Join(parent, "B")
	if err := os.Rename(dirA, dirB); err != nil {
		t.Fatalf("simulate moving the data directory: %v", err)
	}

	moved, err := Load(dirB)
	if err != nil {
		t.Fatalf("Load(B): %v", err)
	}
	got, ok := moved.Get(o.ID)
	if !ok {
		t.Fatalf("organization %q missing after move", o.ID)
	}
	wantPath := filepath.Join(dirB, "orgs", o.ID+".db")
	if p := moved.Path(got); p != wantPath {
		t.Errorf("Path() = %q, want %q (resolved under the new directory)", p, wantPath)
	}
	if strings.Contains(moved.Path(got), dirA) {
		t.Errorf("Path() = %q still references the old directory %q", moved.Path(got), dirA)
	}
}
