package ui

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/store"
)

// SupportURL is the customer-facing help and support page shown in About.
const SupportURL = "https://acuments.com"

// AboutScreen shows product information, version metadata, data location,
// backup/restore actions, and upgrade safety notes.
type AboutScreen struct {
	win     fyne.Window
	fyneApp fyne.App
	db      *store.DB
	dbPath  string
	status  *widget.Label
	root    fyne.CanvasObject
}

// NewAboutScreen builds the About screen. dbPath is the live database file
// path (empty for :memory: test databases).
func NewAboutScreen(win fyne.Window, fyneApp fyne.App, db *store.DB, dbPath string) *AboutScreen {
	a := &AboutScreen{win: win, fyneApp: fyneApp, db: db, dbPath: dbPath}
	a.build()
	return a
}

func (a *AboutScreen) Container() fyne.CanvasObject { return a.root }

func (a *AboutScreen) build() {
	meta := a.fyneApp.Metadata()
	appName := meta.Name
	if appName == "" {
		appName = "Invoice Generator"
	}
	version := meta.Version
	if version == "" {
		version = "0.1.0"
	}
	build := meta.Build
	if build == 0 {
		build = 1
	}

	a.status = widget.NewLabel("")
	a.status.Importance = widget.LowImportance

	productBlurb := wrappedHint(
		"A self-contained desktop utility for Indian GST tax invoices — export under LUT, export with IGST, and domestic CGST/SGST or IGST. " +
			"Configure company, bank, and numbering once; every invoice inherits those defaults. PDFs are written to your chosen output folder.",
	)

	versionLine := mutedBodyText(fmt.Sprintf("Version %s (build %d)", version, build))

	supportHL := widget.NewHyperlink("Support & documentation", mustParseURL(SupportURL))

	dbDisplay := a.dbPath
	if dbDisplay == "" || dbDisplay == ":memory:" {
		dbDisplay = "In-memory (not persisted)"
	}
	dbPathLabel := widget.NewLabel(dbDisplay)
	dbPathLabel.Wrapping = fyne.TextWrapWord

	backupBtn := widget.NewButtonWithIcon("Back up data…", theme.DocumentSaveIcon(), a.backupData)
	restoreBtn := widget.NewButtonWithIcon("Restore from backup…", theme.ContentUndoIcon(), a.restoreData)

	upgradeNote := wrappedHint(
		"Software updates open your existing database and apply any schema changes automatically. " +
			"Settings and invoices are stored as JSON snapshots, so new app versions add fields without rewriting old records. " +
			"Back up before major upgrades if you want an extra safety copy.",
	)

	aboutBody := settingsSection(
		"About this software",
		"Product information and where to get help.",
		stack(
			spaceMd,
			titleText(appName),
			productBlurb,
			versionLine,
			row(spaceSm, supportHL),
		),
	)

	dataBody := settingsSection(
		"Your data",
		"All invoices, customers, items, and settings live in one SQLite database file. Copy it to a USB drive or cloud folder to keep a backup.",
		stack(
			spaceMd,
			field("Database file", dbPathLabel),
			hintText("Generated PDFs are stored separately in the output folder you choose under Settings → Company."),
			row(spaceSm, backupBtn, restoreBtn),
			upgradeNote,
		),
	)

	header := pageHeader(
		"About",
		"Software information, support, and data backup.",
		nil,
	)

	content := maxWidthCenter(
		settingsContentMaxWidth,
		stack(spaceXl, aboutBody, dataBody),
	)

	a.root = container.NewBorder(
		header,
		actionBar(a.status, nil),
		nil, nil,
		padXY(spaceLg, spaceXl, container.NewVScroll(content)),
	)
}

func (a *AboutScreen) backupData() {
	if a.dbPath == "" || a.dbPath == ":memory:" {
		a.setStatus("Backup is not available for in-memory databases.", widget.DangerImportance)
		return
	}
	defaultName := fmt.Sprintf("invoicer-backup-%s.db", time.Now().Format("2006-01-02"))
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		path := writer.URI().Path()
		writer.Close()

		if err := a.db.BackupTo(path); err != nil {
			a.setStatus(fmt.Sprintf("Backup failed: %v", err), widget.DangerImportance)
			return
		}
		a.setStatus(fmt.Sprintf("Backup saved to %s", path), widget.SuccessImportance)
	}, a.win)
	d.SetFileName(defaultName)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".db"}))
	d.Show()
}

func (a *AboutScreen) restoreData() {
	if a.dbPath == "" || a.dbPath == ":memory:" {
		a.setStatus("Restore is not available for in-memory databases.", widget.DangerImportance)
		return
	}
	dialog.ShowConfirm(
		"Restore from backup",
		"This replaces your current database with the chosen backup file. "+
			"Any invoices or settings changed since that backup will be lost. "+
			"The app will close after restore — reopen it to continue.",
		func(ok bool) {
			if !ok {
				return
			}
			a.pickAndRestore()
		},
		a.win,
	)
}

func (a *AboutScreen) pickAndRestore() {
	d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil || rc == nil {
			return
		}
		backupPath := rc.URI().Path()
		rc.Close()

		if err := verifySQLiteFile(backupPath); err != nil {
			a.setStatus(fmt.Sprintf("Invalid backup file: %v", err), widget.DangerImportance)
			return
		}

		if err := a.db.Close(); err != nil {
			a.setStatus(fmt.Sprintf("Could not close database: %v", err), widget.DangerImportance)
			return
		}

		if err := copyFile(backupPath, a.dbPath); err != nil {
			a.setStatus(fmt.Sprintf("Restore failed: %v", err), widget.DangerImportance)
			return
		}

		dialog.ShowInformation(
			"Restore complete",
			"Your data was restored from the backup.\nPlease close and reopen the application.",
			a.win,
		)
		a.fyneApp.Quit()
	}, a.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".db"}))
	d.Show()
}

func (a *AboutScreen) setStatus(msg string, imp widget.Importance) {
	a.status.SetText(msg)
	a.status.Importance = imp
	a.status.Refresh()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		return &url.URL{Scheme: "https", Host: "localhost"}
	}
	return u
}

func verifySQLiteFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(f, header); err != nil {
		return fmt.Errorf("file too small")
	}
	if string(header) != "SQLite format 3\x00" {
		return fmt.Errorf("not a SQLite database")
	}
	return nil
}
