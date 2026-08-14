package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withRedirectedHome points os.UserHomeDir() at a scratch directory on every
// platform (HOME on Unix, USERPROFILE on Windows).
func withRedirectedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// withRedirectedAppDir points appDir() / os.UserConfigDir() at a scratch
// directory instead of the real OS application-data location.
func withRedirectedAppDir(t *testing.T) string {
	t.Helper()
	home := withRedirectedHome(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv(EnvDBPath, "")
	dir, err := appDir()
	if err != nil {
		t.Fatalf("appDir: %v", err)
	}
	return dir
}

func skipUnlessChmodReadOnlyEnforced(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory chmod read-only is not enforced on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
}
