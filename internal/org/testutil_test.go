package org

import (
	"os"
	"runtime"
	"testing"
)

func skipUnlessChmodReadOnlyEnforced(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory chmod read-only is not enforced on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
}
