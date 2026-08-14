// Package org manages the set of organizations (separate businesses) the app
// can invoice for, and which one is currently open.
//
// Each organization owns its own SQLite database file. That is a deliberate
// choice over an org_id column on every table: the organizations a user runs
// here are separate legal entities with their own GSTIN, bank account, invoice
// number series, customers and filing obligations, and nothing about one
// belongs in another's returns. File-per-organization makes cross-contamination
// structurally impossible rather than something every query has to remember to
// filter for, keeps each number series naturally independent, and lets the
// existing single-file backup/restore work per organization unchanged.
//
// The registry itself is a small JSON file (organizations.json) sitting next to
// the database files in the application data directory.
package org

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RegistryFileName is the JSON file, inside the registry directory, that
// records the known organizations and which one is active.
const RegistryFileName = "organizations.json"

// orgsSubdir holds the database file of every organization created through
// this package. The organization adopted from a pre-multi-org install keeps
// its database where it already was (see AdoptLegacy) rather than being moved.
const orgsSubdir = "orgs"

// ErrLastOrganization is returned by Remove when asked to remove the only
// remaining organization. The app always has exactly one organization open, so
// there is no meaningful state with zero of them.
var ErrLastOrganization = errors.New("org: cannot remove the last organization")

// Organization is one business the user invoices for.
type Organization struct {
	// ID is a stable slug derived from the name at creation time. It never
	// changes, so a rename cannot orphan the database file or invalidate
	// ActiveID.
	ID string `json:"id"`
	// Name is the display name, shown in the switcher and the window title.
	Name string `json:"name"`
	// DBFile is the organization's SQLite file, stored relative to the
	// registry directory so the whole data folder stays relocatable (a user
	// syncing it between machines, or restoring it under a different home
	// directory, must not end up with dangling absolute paths).
	DBFile string `json:"dbFile"`
}

// Registry is the on-disk set of organizations plus the active selection.
type Registry struct {
	Organizations []Organization `json:"organizations"`
	ActiveID      string         `json:"activeId"`

	// dir is the directory the registry was loaded from; it is where the
	// JSON file lives and what DBFile paths resolve against. Unexported so
	// it never round-trips through the JSON.
	dir string
}

// Load reads the registry from dir. A missing file is not an error: it yields
// an empty registry, which is exactly the state a first run (or a fresh
// install) should see. Call AdoptLegacy or Add to populate it.
func Load(dir string) (*Registry, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("org: registry directory is required")
	}
	r := &Registry{dir: dir}

	raw, err := os.ReadFile(filepath.Join(dir, RegistryFileName))
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("org: read registry: %w", err)
	}
	if err := json.Unmarshal(raw, r); err != nil {
		return nil, fmt.Errorf("org: parse registry: %w", err)
	}

	// Validate every entry before handing the registry to the app. The file
	// is plain JSON in a user-writable directory, so a hand-edited or
	// corrupted DBFile must not be able to steer the app at an arbitrary
	// path on disk.
	seen := make(map[string]bool, len(r.Organizations))
	for i, o := range r.Organizations {
		if strings.TrimSpace(o.ID) == "" {
			return nil, fmt.Errorf("org: registry entry %d has no id", i)
		}
		if seen[o.ID] {
			return nil, fmt.Errorf("org: duplicate organization id %q", o.ID)
		}
		seen[o.ID] = true
		if err := validDBFile(o.DBFile); err != nil {
			return nil, fmt.Errorf("org: organization %q: %w", o.ID, err)
		}
	}
	// An ActiveID naming an organization that is no longer listed would
	// leave Active() empty and the app with nothing to open; fall back to
	// the first entry rather than failing to start.
	if _, ok := r.Get(r.ActiveID); !ok {
		r.ActiveID = ""
		if len(r.Organizations) > 0 {
			r.ActiveID = r.Organizations[0].ID
		}
	}
	return r, nil
}

// validDBFile rejects anything that is not a plain relative path inside the
// registry directory.
func validDBFile(p string) error {
	if strings.TrimSpace(p) == "" {
		return errors.New("empty database file")
	}
	clean := filepath.Clean(p)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, string(filepath.Separator)) {
		return fmt.Errorf("database file %q must be relative to the data directory", p)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("database file %q escapes the data directory", p)
	}
	return nil
}

// Dir returns the directory the registry and its database files live in.
func (r *Registry) Dir() string { return r.dir }

// List returns the organizations in registry order. The slice is a copy, so
// callers building UI from it cannot mutate the registry by accident.
func (r *Registry) List() []Organization {
	return append([]Organization(nil), r.Organizations...)
}

// Len reports how many organizations exist.
func (r *Registry) Len() int { return len(r.Organizations) }

// Get returns the organization with the given ID.
func (r *Registry) Get(id string) (Organization, bool) {
	for _, o := range r.Organizations {
		if o.ID == id {
			return o, true
		}
	}
	return Organization{}, false
}

// ByName returns the organization with the given display name, compared
// case-insensitively. Names are kept unique so the switcher is unambiguous.
func (r *Registry) ByName(name string) (Organization, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, o := range r.Organizations {
		if strings.ToLower(o.Name) == want {
			return o, true
		}
	}
	return Organization{}, false
}

// Active returns the currently selected organization.
func (r *Registry) Active() (Organization, bool) { return r.Get(r.ActiveID) }

// Names returns the display names in registry order, for the switcher widget.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.Organizations))
	for _, o := range r.Organizations {
		names = append(names, o.Name)
	}
	return names
}

// Path returns the absolute database path for o.
func (r *Registry) Path(o Organization) string {
	return filepath.Join(r.dir, filepath.FromSlash(o.DBFile))
}

// ActivePath returns the database path of the active organization.
func (r *Registry) ActivePath() (string, bool) {
	o, ok := r.Active()
	if !ok {
		return "", false
	}
	return r.Path(o), true
}

// SetActive selects id as the open organization and persists the choice, so
// the app reopens on the same organization next launch.
func (r *Registry) SetActive(id string) error {
	if _, ok := r.Get(id); !ok {
		return fmt.Errorf("org: unknown organization %q", id)
	}
	if r.ActiveID == id {
		return nil
	}
	prev := r.ActiveID
	r.ActiveID = id
	if err := r.Save(); err != nil {
		// Roll back: a failed Save must not leave the running app's
		// registry claiming a different organization is active than the
		// one actually recorded on disk.
		r.ActiveID = prev
		return err
	}
	return nil
}

// Add registers a new organization under the given display name, allocating a
// unique ID and database file, and persists the registry. It does not create
// the database file — opening it is the caller's job (store.Open creates it),
// which keeps this package free of any database dependency.
//
// The new organization becomes active only if there was none before; adding an
// organization from the manage dialog should not silently move the user off the
// one they are working in.
func (r *Registry) Add(name string) (Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Organization{}, errors.New("org: organization name is required")
	}
	if _, exists := r.ByName(name); exists {
		return Organization{}, fmt.Errorf("org: an organization named %q already exists", name)
	}

	id := r.uniqueID(slugify(name))
	o := Organization{
		ID:     id,
		Name:   name,
		DBFile: orgsSubdir + "/" + id + ".db",
	}

	// Create the subdirectory now so store.Open's file creation cannot fail
	// on a missing parent.
	if err := os.MkdirAll(filepath.Join(r.dir, orgsSubdir), 0o755); err != nil {
		return Organization{}, fmt.Errorf("org: create organizations directory: %w", err)
	}

	// Snapshot what we're about to change, so a failed Save leaves the
	// in-memory registry exactly as it was on disk rather than silently
	// diverging — the caller does get the error, but a long-running app
	// keeps using this *Registry past the error dialog (see the UI's
	// addOrgRow). Organizations is a slice and append may or may not reuse
	// its backing array, so the snapshot has to be a copy, not an alias of
	// what append is about to overwrite.
	prevOrgs := append([]Organization(nil), r.Organizations...)
	prevActive := r.ActiveID

	r.Organizations = append(r.Organizations, o)
	if r.ActiveID == "" {
		r.ActiveID = o.ID
	}
	if err := r.Save(); err != nil {
		r.Organizations = prevOrgs
		r.ActiveID = prevActive
		// Best-effort: if this call is what just created orgs/ (a
		// brand-new registry's first Add), a failed Save should not leave
		// an empty directory behind for an organization that was never
		// actually persisted. os.Remove only succeeds on an empty
		// directory, so this is correctly a no-op when other
		// organizations' database files already live there.
		os.Remove(filepath.Join(r.dir, orgsSubdir))
		return Organization{}, err
	}
	return o, nil
}

// AdoptLegacy registers an existing single-organization database file under
// the given name, without moving it, and makes it active. It is a no-op when
// the registry already has organizations.
//
// This is the upgrade path for installs that predate multi-organization
// support: their invoicer.db keeps its exact location on disk, so the upgrade
// touches no user data at all and a downgrade to an older build still finds
// its database where it left it.
func (r *Registry) AdoptLegacy(name, dbFile string) (Organization, error) {
	if len(r.Organizations) > 0 {
		o, _ := r.Active()
		return o, nil
	}
	if err := validDBFile(dbFile); err != nil {
		return Organization{}, fmt.Errorf("org: adopt legacy database: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "My Organization"
	}
	o := Organization{ID: r.uniqueID(slugify(name)), Name: name, DBFile: dbFile}
	r.Organizations = append(r.Organizations, o)
	r.ActiveID = o.ID
	if err := r.Save(); err != nil {
		return Organization{}, err
	}
	return o, nil
}

// Rename changes an organization's display name. The ID and database file are
// untouched, so renaming is always safe.
func (r *Registry) Rename(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("org: organization name is required")
	}
	if existing, ok := r.ByName(name); ok && existing.ID != id {
		return fmt.Errorf("org: an organization named %q already exists", name)
	}
	for i := range r.Organizations {
		if r.Organizations[i].ID == id {
			prevName := r.Organizations[i].Name
			r.Organizations[i].Name = name
			if err := r.Save(); err != nil {
				// Roll back so the in-memory registry (and hence the
				// switcher, which is still showing the old name) does not
				// diverge from what actually got persisted.
				r.Organizations[i].Name = prevName
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("org: unknown organization %q", id)
}

// Remove drops an organization from the registry and returns the path of the
// database file it left behind.
//
// The file is deliberately NOT deleted: it holds finalised tax invoices that a
// business is legally required to retain, and "remove from the list" must never
// be a one-click way to destroy years of filings. The caller is expected to
// tell the user where the file remains.
func (r *Registry) Remove(id string) (string, error) {
	if len(r.Organizations) <= 1 {
		return "", ErrLastOrganization
	}
	idx := -1
	for i, o := range r.Organizations {
		if o.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("org: unknown organization %q", id)
	}

	path := r.Path(r.Organizations[idx])

	// Snapshot before mutating — see Add for why this has to be a copy, not
	// an alias. Remove is the sharpest case: without the rollback below, a
	// failed Save would leave the organization gone from the running app
	// (and, if it was active, ActiveID pointing elsewhere) while its entry
	// and database file are both still fully intact on disk.
	prevOrgs := append([]Organization(nil), r.Organizations...)
	prevActive := r.ActiveID

	r.Organizations = append(r.Organizations[:idx], r.Organizations[idx+1:]...)
	if r.ActiveID == id {
		r.ActiveID = r.Organizations[0].ID
	}
	if err := r.Save(); err != nil {
		r.Organizations = prevOrgs
		r.ActiveID = prevActive
		return "", err
	}
	return path, nil
}

// Save writes the registry to disk.
func (r *Registry) Save() error {
	if strings.TrimSpace(r.dir) == "" {
		return errors.New("org: registry directory is required")
	}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return fmt.Errorf("org: create data directory: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("org: encode registry: %w", err)
	}

	// Write via a temporary file and rename so an interrupted write cannot
	// leave a truncated registry behind — that would lose the user's list of
	// businesses even though every database file is still perfectly intact.
	final := filepath.Join(r.dir, RegistryFileName)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("org: write registry: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("org: replace registry: %w", err)
	}
	return nil
}

// uniqueID returns base, or base-2/base-3/... if base is already taken.
func (r *Registry) uniqueID(base string) string {
	if _, taken := r.Get(base); !taken {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if _, taken := r.Get(candidate); !taken {
			return candidate
		}
	}
}

// slugify reduces a display name to a filesystem- and JSON-safe identifier:
// lowercase ASCII alphanumerics with single dashes between runs of anything
// else. A name with no usable characters at all (e.g. one written entirely in
// a non-Latin script) still needs an ID, so it falls back to "org" and gets
// uniquified by the caller.
func slugify(name string) string {
	var b strings.Builder
	dashPending := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if dashPending && b.Len() > 0 {
				b.WriteByte('-')
			}
			dashPending = false
			b.WriteRune(r)
		default:
			dashPending = true
		}
	}
	s := b.String()
	const maxLen = 40
	if len(s) > maxLen {
		s = strings.TrimRight(s[:maxLen], "-")
	}
	if s == "" {
		return "org"
	}
	return s
}
