package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"

	"dock-invoice/internal/model"
	"dock-invoice/internal/org"
	"dock-invoice/internal/pdf"
	"dock-invoice/internal/store"
)

// App wires the five screens (Invoices, Invoice editor, Settings, Masters,
// About) together over a single *store.DB, and owns the Fyne application/window
// lifecycle.
//
// With an organization registry attached (see NewWorkspaceApp) the App can swap
// that *store.DB for another organization's at runtime: switchOrg reopens the
// database and rebuilds every screen over it.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	db      *store.DB

	settings model.Settings

	tabs           *container.AppTabs
	invoicesTab    *container.TabItem
	editorTab      *container.TabItem
	settingsTab    *container.TabItem
	mastersTab     *container.TabItem
	aboutTab       *container.TabItem
	invoicesScreen *InvoicesScreen
	dbPath         string

	// orgs is the organization registry, or nil in single-database mode
	// (INVOICER_DB_PATH set, or a test constructing the App directly over
	// one *store.DB). Settings hides its Organizations button when it is nil.
	orgs *org.Registry
	// editorOpen tracks whether the Editor tab holds a live invoice rather
	// than the placeholder, so switching organizations can warn before
	// discarding an unsaved draft.
	editorOpen bool
}

// NewApp builds the application over db (loading or seeding settings),
// creating a real desktop Fyne application.
func NewApp(db *store.DB, dbPath string) (*App, error) {
	return NewAppWithFyneApp(app.NewWithID("com.dock.invoice.desktop"), db, dbPath)
}

// NewAppWithFyneApp builds the application over an already-constructed
// fyne.App. This indirection exists so tests can pass in a headless
// fyne.io/fyne/v2/test app instead of a real one — creating a second real
// (e.g. glfw-backed) app on top of the test driver would try to open an
// actual OS window and hang in a headless/CI environment.
//
// The App runs in single-database mode: no organization switcher.
func NewAppWithFyneApp(fyneApp fyne.App, db *store.DB, dbPath string) (*App, error) {
	return newApp(fyneApp, db, dbPath, nil)
}

// NewWorkspaceApp builds the application over an opened Workspace, creating a
// real desktop Fyne application. This is what main.go calls: when the
// workspace carries an organization registry the window gains a switcher.
func NewWorkspaceApp(ws *Workspace) (*App, error) {
	return NewWorkspaceAppWithFyneApp(app.NewWithID("com.dock.invoice.desktop"), ws)
}

// NewWorkspaceAppWithFyneApp is NewWorkspaceApp over a caller-supplied
// fyne.App, for the same headless-test reason as NewAppWithFyneApp.
func NewWorkspaceAppWithFyneApp(fyneApp fyne.App, ws *Workspace) (*App, error) {
	if ws == nil || ws.DB == nil {
		return nil, fmt.Errorf("ui: workspace has no open database")
	}
	return newApp(fyneApp, ws.DB, ws.Path, ws.Orgs)
}

func newApp(fyneApp fyne.App, db *store.DB, dbPath string, orgs *org.Registry) (*App, error) {
	if icon := AppIconResource(); icon != nil {
		fyneApp.SetIcon(icon)
	}

	settings, err := loadOrSeedSettings(db)
	if err != nil {
		return nil, err
	}

	fyneApp.Settings().SetTheme(NewTheme())
	win := fyneApp.NewWindow("Invoices")
	win.Resize(fyne.NewSize(1180, 800))
	win.SetMaster()
	if icon := fyneApp.Icon(); icon != nil {
		win.SetIcon(icon)
	}

	a := &App{fyneApp: fyneApp, win: win, db: db, dbPath: dbPath, settings: settings, orgs: orgs}
	a.build()
	a.applyWindowTitle()
	return a, nil
}

// loadOrSeedSettings reads the settings record from db, seeding and persisting
// first-run defaults when there is none. Shared by startup and by switchOrg,
// since a newly created organization's database is itself a first run.
func loadOrSeedSettings(db *store.DB) (model.Settings, error) {
	settings, ok, err := db.GetSettings()
	if err != nil {
		return model.Settings{}, fmt.Errorf("ui: load settings: %w", err)
	}
	if !ok {
		settings = defaultSettings()
		// Persist the seeded defaults immediately: otherwise they exist
		// only in this in-memory struct, and disappear (silently
		// regenerating from scratch) if the user quits before ever
		// pressing "Save Settings" in the Settings tab. Any other code
		// path that reads settings straight from the DB (rather than
		// through this running App) would also see nothing. A failure
		// here shouldn't stop the app from starting — the in-memory
		// defaults are still perfectly usable for this session, so we
		// degrade gracefully and just leave them unsaved.
		_ = db.SaveSettings(settings)
	}
	return settings, nil
}

// Close releases the App's database handle. main.go defers this rather than
// closing the database it opened, because switchOrg replaces that handle: the
// database open at exit is not necessarily the one startup opened.
func (a *App) Close() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}

// defaultSettings seeds sensible first-run values: numbering patterns and
// declaration text matching the reference sample, and typical entry
// defaults.
func defaultSettings() model.Settings {
	return model.Settings{
		Defaults: model.Defaults{
			HSNSAC:          "998314",
			Unit:            "UNT",
			TaxRatePct:      1800,
			PaymentTermDays: 15,
			Currency:        "USD",
			CopyType:        "ORIGINAL",
		},
		NumberPatterns: map[model.InvoiceType]string{
			model.InvoiceExportLUT:  "AEX{FY}-{SEQ}",
			model.InvoiceExportIGST: "AEXI{FY}-{SEQ}",
			model.InvoiceDomestic:   "DOM{FY}-{SEQ}",
		},
		Declarations: map[model.InvoiceType]string{
			model.InvoiceExportLUT:  pdf.DeclarationLUT,
			model.InvoiceExportIGST: pdf.DeclarationIGST,
			model.InvoiceDomestic:   pdf.DeclarationDomestic,
		},
		LastFXFactor: "1",
	}
}

func (a *App) build() {
	a.invoicesScreen = NewInvoicesScreen(a.win, a.db, a.getSettings, a.openEditor, a.renderAndSave)

	editorPlaceholder := emptyState(
		"No invoice open",
		"Use New invoice, Clone, or Edit from the Invoices tab to start drafting.",
		nil,
	)

	a.invoicesTab = container.NewTabItemWithIcon("Invoices", theme.DocumentIcon(), a.invoicesScreen.Container())
	a.editorTab = container.NewTabItemWithIcon("Editor", theme.DocumentCreateIcon(), editorPlaceholder)
	a.settingsTab = container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), a.buildSettingsContainer())
	a.mastersTab = container.NewTabItemWithIcon("Masters", theme.StorageIcon(), NewMastersScreen(a.win, a.db).Container())
	a.aboutTab = container.NewTabItemWithIcon("About", theme.InfoIcon(), NewAboutScreen(a.win, a.fyneApp, a.db, a.dbPath).Container())

	a.tabs = container.NewAppTabs(a.invoicesTab, a.editorTab, a.settingsTab, a.mastersTab, a.aboutTab)
	a.tabs.SetTabLocation(container.TabLocationTop)

	// buildShell returns the bare tabs in single-database mode, so that
	// layout is byte-identical to what it was before organizations existed.
	a.win.SetContent(a.buildShell())
}

// applyWindowTitle names the window after the open organization, so several
// windows of the same app (or a screenshot in a support thread) can be told
// apart at a glance.
func (a *App) applyWindowTitle() {
	if a.orgs == nil {
		a.win.SetTitle("Invoices")
		return
	}
	if o, ok := a.orgs.Active(); ok {
		a.win.SetTitle("Invoices — " + o.Name)
		return
	}
	a.win.SetTitle("Invoices")
}

func (a *App) getSettings() model.Settings { return a.settings }

func (a *App) buildSettingsContainer() fyne.CanvasObject {
	// Hand the Settings screen an independent copy: model.Settings.
	// NumberPatterns/.Declarations are maps, so a plain struct copy of
	// a.settings would still share the same underlying maps. The screen's
	// per-keystroke handlers write straight into those maps (see
	// patternEntry/declEntry in settings.go), which would otherwise take
	// effect in the running app immediately instead of only after "Save
	// Settings" is pressed.
	screen := NewSettingsScreen(a.win, cloneSettings(a.settings), a.saveSettings)
	return screen.Container()
}

func (a *App) saveSettings(s model.Settings) error {
	if err := a.db.SaveSettings(s); err != nil {
		return err
	}
	// Clone before adopting s: s is the Settings screen's own working
	// copy, and that screen widget is built once and kept alive for the
	// life of the app (see buildSettingsContainer), so its map fields
	// keep receiving live keystroke edits after this point too. Without
	// cloning, a.settings would alias those same maps and the "live edit
	// before Save" bug this guards against would resurface on the very
	// next keystroke in the Settings tab.
	a.settings = cloneSettings(s)
	return nil
}

// cloneSettings returns a deep copy of s, so a caller that hands s off to
// code mutating it in place (e.g. the Settings screen's live entry
// widgets) can do so without those edits leaking back into the original
// until the caller explicitly adopts the copy. model.Settings' only
// reference-typed fields are the NumberPatterns/Declarations maps and
// Company.AddressLines (see internal/model/types.go); every other field is
// a plain value already isolated by Go's normal struct-copy semantics.
func cloneSettings(s model.Settings) model.Settings {
	if s.Company.AddressLines != nil {
		s.Company.AddressLines = append([]string(nil), s.Company.AddressLines...)
	}
	if s.NumberPatterns != nil {
		np := make(map[model.InvoiceType]string, len(s.NumberPatterns))
		for k, v := range s.NumberPatterns {
			np[k] = v
		}
		s.NumberPatterns = np
	}
	if s.Declarations != nil {
		decl := make(map[model.InvoiceType]string, len(s.Declarations))
		for k, v := range s.Declarations {
			decl[k] = v
		}
		s.Declarations = decl
	}
	return s
}

// openEditor swaps the Editor tab's content for a fresh Editor bound to
// state, and switches to it. Used by New/Clone/Edit in the Invoices screen.
// Masters customers/items are loaded here so a user who just edited Masters
// sees the fresh list the next time they open an invoice — the editor holds
// a snapshot for the lifetime of that editing session only.
func (a *App) openEditor(state *EditorState) {
	// Always re-stamp company/bank from live Settings so an Edit of an
	// older invoice (or one created before Settings were filled) picks
	// up GSTIN/State/PAN/logo for the next save + PDF.
	state.ApplySettings(a.settings)
	masters := loadEditorMasters(a.db)
	ed := NewEditor(state, a.settings, masters, func(inv model.Invoice) error {
		// Persist letterhead from current Settings into the snapshot so
		// later re-exports still carry company details even offline.
		stampInvoiceFromSettings(&inv, a.settings)
		if err := a.invoicesScreen.SaveFromEditor(inv); err != nil {
			return err
		}
		// The whole point of this app is to produce a PDF, so a
		// successful save always implicitly re-exports one too (see
		// README's "implicit re-export-on-save"). The invoice is
		// already durably saved in the DB by this point, so a
		// render/write failure here must not be reported as a save
		// failure (that would wrongly suggest the user's data entry was
		// lost, and — because it's surfaced via the editor's onSave
		// error path — would strand them on the Editor tab). Report it
		// via a dialog instead, and still return to the Invoices tab
		// since the save itself did succeed.
		if path, err := a.renderAndSave(inv); err != nil {
			a.showError(fmt.Errorf("invoice %s was saved, but PDF export failed: %w", inv.Number, err))
		} else {
			dialog.ShowInformation("Saved", "Invoice "+inv.Number+" saved.\nPDF written to:\n"+path, a.win)
		}
		a.editorOpen = false
		a.tabs.Select(a.invoicesTab)
		return nil
	}, func() {
		a.editorOpen = false
		a.tabs.Select(a.invoicesTab)
	})
	a.editorTab.Content = ed.Container()
	// Tracked so switching organizations can warn before tearing down a
	// draft that has not been saved yet (see confirmOrgSwitch).
	a.editorOpen = true
	a.tabs.Refresh()
	a.tabs.Select(a.editorTab)
}

// editorMastersSource is the tiny store surface openEditor needs to seed
// the editor's customer/item pickers. *store.DB satisfies it.
type editorMastersSource interface {
	ListCustomers() ([]model.Customer, error)
	ListItems() ([]model.Item, error)
}

// loadEditorMasters returns the current Masters lists for the editor
// pickers. A List error yields empty slices rather than failing openEditor:
// free-text entry still works, and a dialoged failure would be worse than a
// temporarily empty picker (the user can reopen the editor after fixing the
// underlying store problem).
func loadEditorMasters(db editorMastersSource) EditorMasters {
	var m EditorMasters
	if db == nil {
		return m
	}
	if cs, err := db.ListCustomers(); err == nil {
		m.Customers = cs
	}
	if its, err := db.ListItems(); err == nil {
		m.Items = its
	}
	return m
}

// renderAndSave renders inv to PDF and writes it to the configured output
// directory (falling back to defaultOutputDir when unset), returning the
// path written. Used by both the editor's implicit re-export-on-save and
// the Invoices screen's explicit "Re-export PDF" / "View PDF" buttons.
// Current Settings company/bank/logo are stamped onto inv before render so
// letterhead fields always match the Settings tab.
func (a *App) renderAndSave(inv model.Invoice) (string, error) {
	stampInvoiceFromSettings(&inv, a.settings)
	data, err := pdf.Render(&inv)
	if err != nil {
		return "", err
	}
	outDir := a.settings.OutputDir
	if outDir == "" {
		outDir, err = defaultOutputDir()
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, sanitizeFilename(inv.Number)+".pdf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// defaultOutputDir returns the directory PDFs are written to when the user
// hasn't chosen one in Settings: <home>/Documents/Invoices, created on
// demand. This used to fall back to ".", but a double-clicked, packaged
// macOS .app runs with its working directory set to "/", so that silently
// tried to write into the filesystem root and failed with a permission
// error the user had no way to make sense of. A real, writable,
// user-visible folder under Documents is both discoverable and safe to
// write to without special permissions.
func defaultOutputDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for default output folder: %w", err)
	}
	return filepath.Join(home, "Documents", "Invoices"), nil
}

func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "invoice"
	}
	return string(out)
}

// Run shows the main window and blocks until it is closed.
func (a *App) Run() {
	a.win.ShowAndRun()
}

// Window exposes the main window, e.g. for tests that need to drive it
// headlessly via fyne.io/fyne/v2/test.
func (a *App) Window() fyne.Window { return a.win }

// showError is a small helper for surfacing unexpected errors to the user
// via a dialog rather than only a status label.
func (a *App) showError(err error) {
	if err == nil {
		return
	}
	dialog.ShowError(err, a.win)
}
