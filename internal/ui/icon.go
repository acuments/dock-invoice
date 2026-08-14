package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed assets/appicon.png
var appIconPNG []byte

// AppIconResource returns the embedded application icon. Set on fyne.App and
// windows so the UI shows branding; macOS Dock also needs a packaged .app
// bundle (fyne package) — a plain go run / go build binary shows the generic
// "exec" icon regardless.
func AppIconResource() fyne.Resource {
	if len(appIconPNG) == 0 {
		return nil
	}
	return fyne.NewStaticResource("appicon.png", appIconPNG)
}
