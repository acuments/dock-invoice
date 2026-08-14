package ui

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openFile opens path with the OS default application (Preview on macOS, etc.).
// Used by the Invoices "View PDF" action.
func openFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	return nil
}
