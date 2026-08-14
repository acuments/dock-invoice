package ui

import (
	"fmt"
	"strings"

	"dock-invoice/internal/org"
	"dock-invoice/internal/store"
)

// LegacyDBFileName is the database file every install used before
// multi-organization support. It stays the first organization's file so an
// upgrade moves no user data.
const LegacyDBFileName = "invoicer.db"

// DefaultOrgName names the organization created on a genuinely fresh install,
// where there is no company name to borrow yet.
const DefaultOrgName = "My Organization"

// Workspace is an opened data set: the active organization's database, plus
// the registry it came from.
type Workspace struct {
	// DB is the active organization's database, already open.
	DB *store.DB
	// Path is DB's file path (empty for in-memory databases).
	Path string
	// Orgs is the organization registry. It is nil in single-database mode
	// — see OpenWorkspace — and the organization switcher is hidden then.
	Orgs *org.Registry
}

// Close closes the workspace's database.
func (w *Workspace) Close() error {
	if w == nil || w.DB == nil {
		return nil
	}
	return w.DB.Close()
}

// OpenWorkspace resolves which data to open and opens it.
//
// When INVOICER_DB_PATH is set the app runs in single-database mode: that
// variable names one specific file, and a caller who pinned a file (a test, a
// script, someone keeping their data on a USB stick) means that file — not
// "whichever organization happened to be active last". The organization
// switcher is hidden in that mode.
//
// Otherwise it loads the organization registry from the application data
// directory, adopting any pre-multi-organization database as the first
// organization, and opens whichever organization is active.
func OpenWorkspace() (*Workspace, error) {
	if p := envDBPath(); p != "" {
		db, err := store.Open(p)
		if err != nil {
			return nil, fmt.Errorf("open database at %s: %w", p, err)
		}
		return &Workspace{DB: db, Path: p}, nil
	}

	dir, err := appDir()
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	reg, err := org.Load(dir)
	if err != nil {
		return nil, err
	}
	if reg.Len() == 0 {
		if _, err := reg.AdoptLegacy(legacyOrgName(reg), LegacyDBFileName); err != nil {
			return nil, err
		}
	}

	path, ok := reg.ActivePath()
	if !ok {
		return nil, fmt.Errorf("ui: organization registry has no active organization")
	}
	db, err := store.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open database at %s: %w", path, err)
	}
	return &Workspace{DB: db, Path: path, Orgs: reg}, nil
}

// legacyOrgName picks the display name for the organization adopted from a
// pre-multi-organization install. An upgrading user already told us who they
// are in Settings, so borrow the configured company name rather than labelling
// their established business "My Organization" and making them rename it.
//
// Any failure here is cosmetic — the name is editable in the manage dialog —
// so a database that cannot be opened or read just falls back to the default
// rather than blocking startup.
func legacyOrgName(reg *org.Registry) string {
	legacy := reg.Path(org.Organization{DBFile: LegacyDBFileName})
	db, err := store.Open(legacy)
	if err != nil {
		return DefaultOrgName
	}
	defer db.Close()

	settings, ok, err := db.GetSettings()
	if err != nil || !ok {
		return DefaultOrgName
	}
	if name := strings.TrimSpace(settings.Company.Name); name != "" {
		return name
	}
	return DefaultOrgName
}
