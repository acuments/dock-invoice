package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/model"
)

// TestCloneSettings_MapAndSliceFieldsAreIndependent is a focused unit test
// for the cloneSettings helper introduced to fix bug A4: mutating the
// clone's maps/slices must never affect the original.
func TestCloneSettings_MapAndSliceFieldsAreIndependent(t *testing.T) {
	orig := model.Settings{
		Company: model.Company{
			AddressLines: []string{"Line 1", "Line 2"},
		},
		NumberPatterns: map[model.InvoiceType]string{
			model.InvoiceExportLUT: "AEX{FY}-{SEQ}",
		},
		Declarations: map[model.InvoiceType]string{
			model.InvoiceExportLUT: "original declaration",
		},
	}

	clone := cloneSettings(orig)

	clone.Company.AddressLines[0] = "MUTATED"
	clone.NumberPatterns[model.InvoiceExportLUT] = "MUTATED"
	clone.Declarations[model.InvoiceExportLUT] = "MUTATED"
	clone.NumberPatterns[model.InvoiceDomestic] = "NEW-ENTRY"

	if orig.Company.AddressLines[0] != "Line 1" {
		t.Errorf("mutating clone.Company.AddressLines leaked into orig: %q", orig.Company.AddressLines[0])
	}
	if orig.NumberPatterns[model.InvoiceExportLUT] != "AEX{FY}-{SEQ}" {
		t.Errorf("mutating clone.NumberPatterns leaked into orig: %q", orig.NumberPatterns[model.InvoiceExportLUT])
	}
	if orig.Declarations[model.InvoiceExportLUT] != "original declaration" {
		t.Errorf("mutating clone.Declarations leaked into orig: %q", orig.Declarations[model.InvoiceExportLUT])
	}
	if _, ok := orig.NumberPatterns[model.InvoiceDomestic]; ok {
		t.Error("adding a key to clone.NumberPatterns leaked into orig")
	}
}

// TestApp_SettingsScreen_EditsDoNotMutateAppSettingsUntilSave is the
// regression test for bug A4: editing a numbering-pattern (or declaration)
// field in the Settings tab must not take effect in the running app until
// "Save Settings" is pressed, matching how the plain string fields already
// behaved. Before the fix, buildSettingsContainer passed a.settings
// straight to NewSettingsScreen, and since NumberPatterns/Declarations are
// maps, the screen's per-keystroke handlers mutated a.settings' maps
// directly and immediately.
func TestApp_SettingsScreen_EditsDoNotMutateAppSettingsUntilSave(t *testing.T) {
	a := newTestApp(t)

	original := a.settings.NumberPatterns[model.InvoiceExportLUT]
	if original == "" {
		t.Fatal("test setup: expected a seeded default pattern for export_lut")
	}

	patternEntry := findEntryByPlaceholder(t, a.settingsTab.Content, "AEX{FY}-{SEQ}")
	patternEntry.SetText("CHANGED-{SEQ}")

	if got := a.settings.NumberPatterns[model.InvoiceExportLUT]; got != original {
		t.Fatalf("editing the Settings screen mutated a.settings before Save Settings was pressed: got %q, want unchanged %q", got, original)
	}

	saveBtn := findButtonByText(t, a.settingsTab.Content, "Save settings")
	test.Tap(saveBtn)

	if got := a.settings.NumberPatterns[model.InvoiceExportLUT]; got != "CHANGED-{SEQ}" {
		t.Fatalf("after Save Settings, a.settings.NumberPatterns[export_lut] = %q, want %q", got, "CHANGED-{SEQ}")
	}

	// Regression for the second-order aliasing bug on the a.saveSettings
	// path: after a Save, a.settings must not end up aliasing the still-
	// open Settings screen's own working map either. If it did, the very
	// next keystroke on that screen would silently change a.settings again
	// ahead of any further Save.
	patternEntry.SetText("CHANGED-{SEQ}-MORE")
	if got := a.settings.NumberPatterns[model.InvoiceExportLUT]; got != "CHANGED-{SEQ}" {
		t.Fatalf("editing again after Save mutated a.settings before the next Save: got %q, want still %q", got, "CHANGED-{SEQ}")
	}
}

// findEntryByPlaceholder walks root for the first *widget.Entry whose
// placeholder text matches. Used to locate the numbering-pattern fields in
// the Settings screen, which (unlike the declaration fields) have distinct
// placeholders to search on.
func findEntryByPlaceholder(t *testing.T, root any, placeholder string) *widget.Entry {
	t.Helper()
	var found *widget.Entry
	walk(root, func(o any) {
		if found != nil {
			return
		}
		if e, ok := o.(*widget.Entry); ok && e.PlaceHolder == placeholder {
			found = e
		}
	})
	if found == nil {
		t.Fatalf("could not find an entry with placeholder %q", placeholder)
	}
	return found
}
