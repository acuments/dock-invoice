package ui

import "dock-invoice/internal/model"

// stampInvoiceFromSettings copies the live company / bank / logo / signature
// and the matching declaration text from settings onto inv. Always called
// before a PDF is rendered (and before DB save) so the Settings tab is the
// single source of truth for letterhead fields — without this, invoices
// snapshotted while settings were still empty keep printing blanks even after
// the user fills in GSTIN/State/PAN later.
func stampInvoiceFromSettings(inv *model.Invoice, s model.Settings) {
	if inv == nil {
		return
	}
	inv.Seller = sellerFromSettings(s)
	inv.Bank = s.Bank
	inv.IncludeHSNSACSummary = s.IncludeHSNSACSummary
	// Unlike the fields above, LayoutStyle is only a fallback: the editor
	// exposes it as a per-invoice choice (see Editor's layoutSelect), so
	// overwriting it unconditionally here would silently discard that
	// choice on every save and re-export. Only fill it in when the invoice
	// has no explicit choice of its own.
	if inv.LayoutStyle == "" {
		inv.LayoutStyle = model.NormalizeLayoutStyle(s.LayoutStyle)
	}
	if decl, ok := s.Declarations[inv.Type]; ok && decl != "" {
		inv.Notes = decl
	}
}

// sellerFromSettings returns the Company snapshot the PDF/letterhead should
// print, merging the top-level LogoPath/SignaturePath fields that the
// Settings UI also writes (older data and the dual-field layout can leave
// either side empty).
func sellerFromSettings(s model.Settings) model.Company {
	c := s.Company
	if c.LogoPath == "" {
		c.LogoPath = s.LogoPath
	}
	if c.SignaturePath == "" {
		c.SignaturePath = s.SignaturePath
	}
	return c
}

// ApplySettings stamps the current company/bank onto an in-progress editor
// so a New/Edit/Clone session always starts from today's Settings, not a
// stale empty snapshot from an older save.
func (s *EditorState) ApplySettings(settings model.Settings) {
	s.Seller = sellerFromSettings(settings)
	s.Bank = settings.Bank
}
