package ui

import (
	"os"
	"path/filepath"
	"strings"
)

// EnvDBPath, when set, overrides the resolved database path — used by tests
// (and anyone scripting the app) to point at a scratch database instead of
// the OS application-data directory.
const EnvDBPath = "INVOICER_DB_PATH"

// AppDirName is the folder created under the OS config directory.
const AppDirName = "InvoiceGenerator"

// envDBPath returns the INVOICER_DB_PATH override, or "" when it is unset or
// blank. Blank is treated as unset so an exported-but-empty variable cannot
// resolve the database to the current directory.
func envDBPath() string {
	return strings.TrimSpace(os.Getenv(EnvDBPath))
}

// ResolveDBPath returns the SQLite database path to use: the value of the
// INVOICER_DB_PATH environment variable if set (non-empty), otherwise
// <os.UserConfigDir()>/InvoiceGenerator/invoicer.db, creating the directory
// if needed.
//
// This resolves the single default database. Multi-organization startup goes
// through OpenWorkspace instead, which consults the organization registry in
// the same directory.
func ResolveDBPath() (string, error) {
	if p := envDBPath(); p != "" {
		return p, nil
	}
	dir, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "invoicer.db"), nil
}

// appDir returns (creating if necessary) <os.UserConfigDir()>/InvoiceGenerator.
func appDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, AppDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
