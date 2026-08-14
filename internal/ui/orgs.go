package ui

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/org"
	"dock-invoice/internal/store"
)

// buildOrgBar builds the organization button, right-aligned and vertically
// centred on the tab strip, or returns nil in single-database mode (where
// there is nothing to manage).
//
// The button sits alone: the switcher that used to fill this row stretched
// across the whole window for a control used a handful of times a year, and
// duplicated what the manage dialog's per-row Open already does.
//
// It is returned for overlaying (see buildShell) rather than stacking above
// the tabs, so it costs no vertical space at all — the tab strip is already a
// mostly-empty row, and a second row to hold one button was the whole reason
// the bar looked wrong.
func (a *App) buildOrgBar() fyne.CanvasObject {
	if a.orgs == nil {
		return nil
	}

	manageBtn := widget.NewButtonWithIcon("Organizations…", theme.StorageIcon(), a.showManageOrgs)
	manageBtn.Importance = widget.LowImportance

	// Border's right edge takes the button at its natural width and leaves
	// the rest of the row empty, which is what pins it to the far edge;
	// NewCenter then centres it against the tab labels rather than letting
	// it ride the top of the row.
	bar := container.NewBorder(nil, nil, nil, container.NewCenter(manageBtn), nil)
	return padXY(0, spaceXl, bar)
}

// buildShell composes the tab strip with the organization button laid over its
// right-hand end, on the same line.
//
// A Stack rather than a Border: AppTabs draws its own full-width strip, so the
// only way to put anything on that line is on top of it. Plain containers are
// not tappable in Fyne, so the overlay's empty area does not swallow clicks
// meant for the tabs beneath — only the button itself takes them. The tabs are
// five short labels on the left of a wide window, so nothing collides.
func (a *App) buildShell() fyne.CanvasObject {
	bar := a.buildOrgBar()
	if bar == nil {
		return a.tabs
	}
	// The overlay is confined to the strip's height by a VBox: the row takes
	// its MinSize and the spacer swallows everything below it, so the button
	// cannot end up floating over screen content.
	return container.NewStack(a.tabs, container.NewVBox(bar, layout.NewSpacer()))
}

// confirmOrgSwitch switches to o, first asking about any unsaved draft in the
// Editor tab — switching organizations closes that database, so an in-progress
// invoice would be discarded silently otherwise.
func (a *App) confirmOrgSwitch(o org.Organization) {
	if a.orgs == nil {
		return
	}
	if a.orgs.ActiveID == o.ID {
		return
	}
	if !a.editorOpen {
		a.doSwitchOrg(o)
		return
	}
	dialog.ShowConfirm(
		"Discard open invoice?",
		fmt.Sprintf("The invoice open in the Editor tab has not been saved.\nSwitching to %s will discard it.", o.Name),
		func(ok bool) {
			if !ok {
				return
			}
			a.doSwitchOrg(o)
		},
		a.win,
	)
}

// doSwitchOrg performs the switch and reports any failure to the user.
func (a *App) doSwitchOrg(o org.Organization) {
	if err := a.switchOrg(o.ID); err != nil {
		a.showError(fmt.Errorf("could not open %s: %w", o.Name, err))
	}
}

// switchOrg opens the given organization's database and rebuilds every screen
// over it.
//
// The new database is opened *before* anything is torn down: if opening fails
// (a deleted file, a permissions problem, a database in use), the app must stay
// exactly where it was rather than being left with no open organization.
func (a *App) switchOrg(id string) error {
	if a.orgs == nil {
		return errors.New("ui: no organization registry")
	}
	o, ok := a.orgs.Get(id)
	if !ok {
		return fmt.Errorf("ui: unknown organization %q", id)
	}
	if a.orgs.ActiveID == id {
		return nil
	}

	path := a.orgs.Path(o)
	newDB, err := store.Open(path)
	if err != nil {
		return err
	}
	settings, err := loadOrSeedSettings(newDB)
	if err != nil {
		newDB.Close()
		return err
	}
	if err := a.orgs.SetActive(id); err != nil {
		newDB.Close()
		return err
	}

	old := a.db
	a.db = newDB
	a.dbPath = path
	a.settings = settings
	// The Editor tab's content is rebuilt as the placeholder below, so any
	// draft it held is gone — the flag has to go with it, or the next switch
	// would warn about an invoice that is no longer there.
	a.editorOpen = false

	a.build()
	a.applyWindowTitle()

	if old != nil {
		old.Close()
	}
	return nil
}

// showManageOrgs opens the add / rename / remove dialog.
func (a *App) showManageOrgs() {
	if a.orgs == nil {
		return
	}
	body := container.NewVBox()
	d := dialog.NewCustom("Organizations", "Done", body, a.win)
	d.Resize(fyne.NewSize(560, 460))

	// The dialog's contents are rebuilt from the registry after every
	// mutation, which keeps one rendering path for "what the registry looks
	// like now" instead of patching rows in place.
	var refresh func()
	refresh = func() {
		body.RemoveAll()
		body.Add(wrappedHint(
			"Each organization keeps its own invoices, customers, items, number series and settings in its own database file. " +
				"Nothing is shared between them.",
		))
		body.Add(rule())
		for _, o := range a.orgs.List() {
			body.Add(a.orgRow(o, d, refresh))
		}
		body.Add(rule())
		body.Add(a.addOrgRow(refresh))
		body.Refresh()
	}
	refresh()

	d.Show()
}

// orgRow renders one organization line in the manage dialog.
func (a *App) orgRow(o org.Organization, d *dialog.CustomDialog, refresh func()) fyne.CanvasObject {
	name := widget.NewLabel(o.Name)
	name.TextStyle = fyne.TextStyle{Bold: a.orgs.ActiveID == o.ID}

	var mark fyne.CanvasObject = widget.NewLabel("")
	if a.orgs.ActiveID == o.ID {
		mark = badge("Open", toneGreen)
	}

	openBtn := widget.NewButton("Open", func() {
		d.Hide()
		a.confirmOrgSwitch(o)
	})
	openBtn.Importance = widget.LowImportance
	if a.orgs.ActiveID == o.ID {
		openBtn.Disable()
	}

	renameBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		a.promptRenameOrg(o, refresh)
	})
	renameBtn.Importance = widget.LowImportance

	removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		a.promptRemoveOrg(o, refresh)
	})
	removeBtn.Importance = widget.LowImportance

	return container.NewBorder(nil, nil, mark,
		container.NewHBox(openBtn, renameBtn, removeBtn),
		name,
	)
}

// addOrgRow renders the "add an organization" control.
func (a *App) addOrgRow(refresh func()) fyne.CanvasObject {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("New organization name")

	add := func() {
		name := strings.TrimSpace(entry.Text)
		if name == "" {
			return
		}
		o, err := a.orgs.Add(name)
		if err != nil {
			a.showError(err)
			return
		}
		entry.SetText("")
		refresh()
		// A brand-new organization has no company details yet, and the
		// first thing anyone does with one is fill those in — so offer to
		// go there rather than leaving them to reopen this dialog.
		dialog.ShowConfirm(
			"Open "+o.Name+"?",
			"It starts empty. Open it now to fill in company, bank and numbering settings?",
			func(ok bool) {
				if ok {
					a.confirmOrgSwitch(o)
				}
			},
			a.win,
		)
	}
	entry.OnSubmitted = func(string) { add() }

	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), add)
	addBtn.Importance = widget.HighImportance

	return container.NewBorder(nil, nil, nil, addBtn, entry)
}

func (a *App) promptRenameOrg(o org.Organization, refresh func()) {
	entry := widget.NewEntry()
	entry.SetText(o.Name)
	dialog.ShowForm("Rename organization", "Rename", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Name", entry)},
		func(ok bool) {
			if !ok {
				return
			}
			if err := a.orgs.Rename(o.ID, entry.Text); err != nil {
				a.showError(err)
				return
			}
			a.applyWindowTitle()
			refresh()
		},
		a.win,
	)
}

func (a *App) promptRemoveOrg(o org.Organization, refresh func()) {
	if a.orgs.Len() <= 1 {
		a.showError(org.ErrLastOrganization)
		return
	}
	dialog.ShowConfirm(
		"Remove "+o.Name+"?",
		"This removes it from the list only. Its invoices and data file are left on disk, "+
			"so you can add it back later or archive the file yourself.",
		func(ok bool) {
			if !ok {
				return
			}
			path, err := a.orgs.Remove(o.ID)
			if err != nil {
				a.showError(err)
				return
			}
			// Removing the organization that was open leaves the app
			// pointed at a database it no longer lists, so follow the
			// registry to whichever one it fell back to.
			if a.dbPath != path {
				refresh()
			} else if active, found := a.orgs.Active(); found {
				// SetActive already happened inside Remove, so switchOrg
				// would early-return; go through the same reopen path by
				// forcing the swap here.
				if err := a.reopenActive(active); err != nil {
					a.showError(err)
				}
				refresh()
			}
			dialog.ShowInformation(
				"Removed",
				o.Name+" was removed from the list.\nIts data file remains at:\n"+path,
				a.win,
			)
		},
		a.win,
	)
}

// reopenActive swaps the App onto the registry's current active organization.
// Used after Remove, which has already updated ActiveID and so would make
// switchOrg's "already active" early return skip the actual reopen.
func (a *App) reopenActive(o org.Organization) error {
	path := a.orgs.Path(o)
	newDB, err := store.Open(path)
	if err != nil {
		return err
	}
	settings, err := loadOrSeedSettings(newDB)
	if err != nil {
		newDB.Close()
		return err
	}

	old := a.db
	a.db = newDB
	a.dbPath = path
	a.settings = settings
	a.editorOpen = false

	a.build()
	a.applyWindowTitle()

	if old != nil {
		old.Close()
	}
	return nil
}
