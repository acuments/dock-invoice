package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"dock-invoice/internal/model"
)

// SettingsScreen is the "one-time configuration" screen: every static field
// that appears on the PDF (company, bank, logo/signature, output folder,
// per-series numbering patterns, entry defaults, and the declaration text
// per invoice type) is reachable from here so it never needs retyping per
// invoice.
type SettingsScreen struct {
	win      fyne.Window
	settings model.Settings
	onSave   func(model.Settings) error
	root     fyne.CanvasObject

	logoPathLabel  *widget.Label
	sigPathLabel   *widget.Label
	outputDirLabel *widget.Label
	status         *widget.Label
}

// NewSettingsScreen builds the Settings screen over a copy of settings.
// Nothing is persisted until Save is clicked.
func NewSettingsScreen(win fyne.Window, settings model.Settings, onSave func(model.Settings) error) *SettingsScreen {
	s := &SettingsScreen{win: win, settings: settings, onSave: onSave}
	s.build()
	return s
}

func (s *SettingsScreen) Container() fyne.CanvasObject { return s.root }

func (s *SettingsScreen) build() {
	company := s.buildCompanySection()
	bank := s.buildBankSection()
	defaults := s.buildDefaultsSection()
	numbering := s.buildNumberingSection()
	declarations := s.buildDeclarationsSection()

	// Nested tabs keep each concern on one focused page instead of a wall
	// of stock Fyne forms. Order mirrors the mental model: who you are →
	// how you're paid → defaults → numbering → legal text.
	tabs := container.NewAppTabs(
		container.NewTabItem("Company", company),
		container.NewTabItem("Bank", bank),
		container.NewTabItem("Defaults", defaults),
		container.NewTabItem("Numbering", numbering),
		container.NewTabItem("Declarations", declarations),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	s.status = widget.NewLabel("")
	s.status.Importance = widget.LowImportance

	saveBtn := widget.NewButtonWithIcon("Save settings", theme.DocumentSaveIcon(), func() {
		if s.onSave != nil {
			if err := s.onSave(s.settings); err != nil {
				s.status.SetText(fmt.Sprintf("Save failed: %v", err))
				s.status.Importance = widget.DangerImportance
				s.status.Refresh()
				return
			}
		}
		s.status.Importance = widget.SuccessImportance
		s.status.SetText("Settings saved.")
		s.status.Refresh()
	})
	saveBtn.Importance = widget.HighImportance

	header := pageHeader(
		"Settings",
		"Configure once. Every new invoice inherits company, bank, defaults, and numbering from here.",
		nil,
	)

	s.root = container.NewBorder(
		header,
		actionBar(s.status, saveBtn),
		nil, nil,
		tabStripGutter(tabs),
	)
}

// ---- Company ---------------------------------------------------------------

func (s *SettingsScreen) buildCompanySection() fyne.CanvasObject {
	c := &s.settings.Company

	name := widget.NewEntry()
	name.SetPlaceHolder("Legal business name as on GST registration")
	name.SetText(c.Name)
	name.OnChanged = func(v string) { c.Name = v }

	address := multiLineEntry(strings.Join(c.AddressLines, "\n"), 3)
	address.SetPlaceHolder("Street address, one line per row")
	address.OnChanged = func(v string) { c.AddressLines = splitNonEmptyLines(v) }

	city := widget.NewEntry()
	city.SetPlaceHolder("e.g. Chennai")
	city.SetText(c.City)
	city.OnChanged = func(v string) { c.City = v }

	pincode := widget.NewEntry()
	pincode.SetPlaceHolder("6 digits")
	pincode.SetText(c.Pincode)
	pincode.OnChanged = func(v string) { c.Pincode = v }

	stateCode := widget.NewEntry()
	stateCode.SetPlaceHolder("e.g. 33")
	stateCode.SetText(c.StateCode)
	stateCode.OnChanged = func(v string) { c.StateCode = v }

	stateName := widget.NewEntry()
	stateName.SetPlaceHolder("e.g. Tamil Nadu")
	stateName.SetText(c.StateName)
	stateName.OnChanged = func(v string) { c.StateName = v }

	phone := widget.NewEntry()
	phone.SetPlaceHolder("10-digit mobile")
	phone.SetText(c.Phone)
	phone.OnChanged = func(v string) { c.Phone = v }

	email := widget.NewEntry()
	email.SetPlaceHolder("billing@company.com")
	email.SetText(c.Email)
	email.OnChanged = func(v string) { c.Email = v }

	gstin := widget.NewEntry()
	gstin.SetPlaceHolder("15-character GSTIN")
	gstin.SetText(c.GSTIN)
	gstin.OnChanged = func(v string) { c.GSTIN = v }

	pan := widget.NewEntry()
	pan.SetPlaceHolder("10-character PAN")
	pan.SetText(c.PAN)
	pan.OnChanged = func(v string) { c.PAN = v }

	s.logoPathLabel = widget.NewLabel(firstNonEmpty(c.LogoPath, "No logo chosen"))
	logoBtn := widget.NewButton("Choose…", func() {
		d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			c.LogoPath = rc.URI().Path()
			s.settings.LogoPath = c.LogoPath
			s.logoPathLabel.SetText(c.LogoPath)
		}, s.win)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		d.Show()
	})

	s.sigPathLabel = widget.NewLabel(firstNonEmpty(c.SignaturePath, "No signature chosen"))
	sigBtn := widget.NewButton("Choose…", func() {
		d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			c.SignaturePath = rc.URI().Path()
			s.settings.SignaturePath = c.SignaturePath
			s.sigPathLabel.SetText(c.SignaturePath)
		}, s.win)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		d.Show()
	})

	s.outputDirLabel = widget.NewLabel(firstNonEmpty(s.settings.OutputDir, "Documents/Invoices (default)"))
	outputDirBtn := widget.NewButton("Choose…", func() {
		d := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			s.settings.OutputDir = lu.Path()
			s.outputDirLabel.SetText(s.settings.OutputDir)
		}, s.win)
		d.Show()
	})

	identity := formStack(
		field("Business name", name),
		field("Registered address", address),
		fields([]float32{1.4, 0.8},
			field("City", city),
			field("PIN code", pincode),
		),
		fields([]float32{0.6, 1.4},
			fieldWithHint("State code", stateCode, "GST state code (e.g. 33)"),
			field("State name", stateName),
		),
	)

	contact := formStack(
		fields([]float32{1, 1},
			field("Phone", phone),
			field("Email", email),
		),
		fields([]float32{1.3, 0.9},
			fieldWithHint("GSTIN", gstin, "Printed on every invoice letterhead"),
			field("PAN", pan),
		),
	)

	assets := formStack(
		fieldWithHint("Logo", pathPickerRow(logoBtn, s.logoPathLabel), "PNG or JPEG, used on the PDF header"),
		fieldWithHint("Signature", pathPickerRow(sigBtn, s.sigPathLabel), "PNG or JPEG, used above the authorised signatory"),
		fieldWithHint("PDF output folder", pathPickerRow(outputDirBtn, s.outputDirLabel), "Where Save and View PDF write invoice files"),
	)

	page := formStack(
		settingsSection("Identity", "Letterhead details that every invoice snapshots at save time.", identity),
		settingsSection("Contact & tax IDs", "Shown under your company block on the PDF.", contact),
		settingsSection("Assets & export", "Branding images and the folder PDF files land in.", assets),
	)
	return container.NewVScroll(
		padXY(spaceLg, spaceXl, maxWidthCenter(settingsContentMaxWidth, page)),
	)
}

// ---- Bank ------------------------------------------------------------------

func (s *SettingsScreen) buildBankSection() fyne.CanvasObject {
	b := &s.settings.Bank

	acct := widget.NewEntry()
	acct.SetPlaceHolder("Account number")
	acct.SetText(b.AccountNumber)
	acct.OnChanged = func(v string) { b.AccountNumber = strings.TrimSpace(v) }

	ifsc := widget.NewEntry()
	ifsc.SetPlaceHolder("e.g. SBIN0001234")
	ifsc.SetText(b.IFSC)
	ifsc.OnChanged = func(v string) { b.IFSC = v }

	bankName := widget.NewEntry()
	bankName.SetPlaceHolder("e.g. IDFC First Bank")
	bankName.SetText(b.BankName)
	bankName.OnChanged = func(v string) { b.BankName = v }

	branchName := widget.NewEntry()
	branchName.SetPlaceHolder("e.g. Parrys Branch")
	branchName.SetText(b.BranchName)
	branchName.OnChanged = func(v string) { b.BranchName = v }

	swift := widget.NewEntry()
	swift.SetPlaceHolder("e.g. EXAMPLEBBXXX")
	swift.SetText(b.SwiftCode)
	swift.OnChanged = func(v string) { b.SwiftCode = v }

	upi := widget.NewEntry()
	upi.SetPlaceHolder("e.g. yourname@hdfc")
	upi.SetText(b.UPIID)
	upi.OnChanged = func(v string) { b.UPIID = strings.TrimSpace(v) }

	body := formStack(
		fields([]float32{1.3, 0.9},
			field("Account number", acct),
			field("IFSC", ifsc),
		),
		fields([]float32{1.2, 1},
			field("Bank name", bankName),
			field("Branch", branchName),
		),
		fieldWithHint("SWIFT / BIC", swift, "Required for export invoices receiving foreign remittance"),
		fieldWithHint("UPI ID", upi, "Adds a scannable UPI QR on every invoice PDF with the total pre-filled"),
	)

	page := formStack(
		settingsSection("Bank details", "Printed in the Bank Details box on every PDF. Fill once; change only when your account changes.", body),
	)
	return container.NewVScroll(
		padXY(spaceLg, spaceXl, maxWidthCenter(settingsContentMaxWidth, page)),
	)
}

// ---- Defaults --------------------------------------------------------------

func (s *SettingsScreen) buildDefaultsSection() fyne.CanvasObject {
	def := &s.settings.Defaults

	defHSN := widget.NewEntry()
	defHSN.SetPlaceHolder("e.g. 998314")
	defHSN.SetText(def.HSNSAC)
	defHSN.OnChanged = func(v string) { def.HSNSAC = strings.TrimSpace(v) }

	defUnit := widget.NewEntry()
	defUnit.SetPlaceHolder("e.g. UNT")
	defUnit.SetText(def.Unit)
	defUnit.OnChanged = func(v string) { def.Unit = strings.TrimSpace(v) }

	defTaxRate := newDecimalEntry("18")
	defTaxRate.SetPlaceHolder("18")
	defTaxRate.SetText(bpsToPercentString(def.TaxRatePct))
	defTaxRate.OnChanged = func(v string) { def.TaxRatePct = percentStringToBps(v) }

	defTerms := widget.NewEntry()
	defTerms.SetPlaceHolder("15")
	defTerms.SetText(strconv.Itoa(def.PaymentTermDays))
	defTerms.OnChanged = func(v string) {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			def.PaymentTermDays = n
		}
	}

	defCurrency := widget.NewEntry()
	defCurrency.SetPlaceHolder("USD or INR")
	defCurrency.SetText(def.Currency)
	defCurrency.OnChanged = func(v string) { def.Currency = v }

	defCopyType := widget.NewSelect([]string{"ORIGINAL", "DUPLICATE", "TRIPLICATE"}, func(v string) { def.CopyType = v })
	defCopyType.SetSelected(firstNonEmpty(def.CopyType, "ORIGINAL"))

	lastFX := newDecimalEntry("83.2")
	lastFX.SetPlaceHolder("e.g. 83.2")
	lastFX.SetText(s.settings.LastFXFactor)
	lastFX.OnChanged = func(v string) { s.settings.LastFXFactor = v }

	hsnSummary := widget.NewCheck("Print HSN/SAC summary on domestic invoices", func(v bool) {
		s.settings.IncludeHSNSACSummary = v
	})
	hsnSummary.SetChecked(s.settings.IncludeHSNSACSummary)

	layoutSelect := widget.NewSelect([]string{
		model.LayoutStyleLabel(model.LayoutModern),
		model.LayoutStyleLabel(model.LayoutClassic),
	}, func(v string) {
		if ls, ok := model.LayoutStyleFromLabel(v); ok {
			s.settings.LayoutStyle = ls
		}
	})
	layoutSelect.SetSelected(model.LayoutStyleLabel(model.NormalizeLayoutStyle(s.settings.LayoutStyle)))

	body := formStack(
		fields([]float32{1.2, 0.7, 0.7},
			fieldWithHint("Default HSN / SAC", defHSN, "Prefilled on each new line"),
			field("Unit", defUnit),
			field("Tax rate %", defTaxRate),
		),
		fields([]float32{1, 0.9, 1.1},
			fieldWithHint("Payment terms (days)", defTerms, "Used to seed due date"),
			field("Currency", defCurrency),
			field("Copy type", defCopyType),
		),
		fieldWithHint("Last FX factor (INR per 1 USD)", lastFX, "Remembered between export invoices so you rarely retype conversion"),
		fieldWithHint("", hsnSummary, "Adds the HSN/SAC tax breakdown table at the foot of domestic PDFs. Export invoices are never affected."),
		fieldWithHint("Invoice layout", layoutSelect, "Classic is the fully-ruled black & white GST format; Modern is the colour letterhead. Either works for any invoice type. This is the default for new invoices and for any invoice whose Layout field in the editor was never changed. To switch an existing invoice, open it and change Layout there."),
	)

	page := formStack(
		settingsSection("Line & invoice defaults", "These seed New invoices and new line rows. Change them here — not on every invoice.", body),
	)
	return container.NewVScroll(
		padXY(spaceLg, spaceXl, maxWidthCenter(settingsContentMaxWidth, page)),
	)
}

// ---- Numbering -------------------------------------------------------------

func (s *SettingsScreen) buildNumberingSection() fyne.CanvasObject {
	if s.settings.NumberPatterns == nil {
		s.settings.NumberPatterns = map[model.InvoiceType]string{}
	}
	patternEntry := func(t model.InvoiceType, placeholder string) *widget.Entry {
		e := widget.NewEntry()
		e.SetPlaceHolder(placeholder)
		e.SetText(s.settings.NumberPatterns[t])
		e.OnChanged = func(v string) { s.settings.NumberPatterns[t] = v }
		return e
	}

	body := formStack(
		fieldWithHint("Export (LUT)", patternEntry(model.InvoiceExportLUT, "AEX{FY}-{SEQ}"), "Tokens: {FY} financial year · {SEQ} next sequence"),
		field("Export (IGST)", patternEntry(model.InvoiceExportIGST, "AEXI{FY}-{SEQ}")),
		field("Domestic", patternEntry(model.InvoiceDomestic, "DOM{FY}-{SEQ}")),
	)

	page := formStack(
		settingsSection("Invoice number series", "Each invoice type has its own sequence. Patterns expand when you create a New invoice.", body),
	)
	return container.NewVScroll(
		padXY(spaceLg, spaceXl, maxWidthCenter(settingsContentMaxWidth, page)),
	)
}

// ---- Declarations ----------------------------------------------------------

func (s *SettingsScreen) buildDeclarationsSection() fyne.CanvasObject {
	if s.settings.Declarations == nil {
		s.settings.Declarations = map[model.InvoiceType]string{}
	}
	declEntry := func(t model.InvoiceType) *widget.Entry {
		e := multiLineEntry(s.settings.Declarations[t], 4)
		e.SetPlaceHolder("Legal declaration printed at the foot of the PDF…")
		e.OnChanged = func(v string) { s.settings.Declarations[t] = v }
		return e
	}

	body := formStack(
		field("Export under LUT / bond", declEntry(model.InvoiceExportLUT)),
		rule(),
		field("Export with IGST", declEntry(model.InvoiceExportIGST)),
		rule(),
		field("Domestic supply", declEntry(model.InvoiceDomestic)),
	)

	page := formStack(
		settingsSection("Declarations", "Legal wording stamped onto each invoice type at save. Edit carefully — these appear on customer-facing PDFs.", body),
	)
	return container.NewVScroll(
		padXY(spaceLg, spaceXl, maxWidthCenter(settingsContentMaxWidth, page)),
	)
}

func percentStringToBps(s string) int32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	whole, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		whole, frac = s[:i], s[i+1:]
	}
	w, err := strconv.Atoi(whole)
	if err != nil {
		return 0
	}
	f := 0
	if frac != "" {
		if len(frac) > 2 {
			frac = frac[:2]
		}
		for len(frac) < 2 {
			frac += "0"
		}
		f, _ = strconv.Atoi(frac)
	}
	return int32(w*100 + f)
}
