package ui

import "dock-invoice/internal/model"

// invoiceTypeLabels is the single source of truth mapping each
// model.InvoiceType to the human-readable label shown in the UI. The enum's
// own values (e.g. "export_lut") are a serialisation format, not copy, and
// must never be rendered to the user directly. Keeping the mapping here —
// rather than inline in whichever screen happens to render a type first —
// means the invoices table, the editor's type dropdown and any future
// screen all say the same thing, and a wording change only touches one
// place.
var invoiceTypeLabels = map[model.InvoiceType]string{
	model.InvoiceExportLUT:  "Export (LUT)",
	model.InvoiceExportIGST: "Export (IGST)",
	model.InvoiceDomestic:   "Domestic",
}

// invoiceTypeOrder fixes a stable, sensible display order for the three
// types (used by InvoiceTypeLabels below), independent of Go's randomised
// map iteration order.
var invoiceTypeOrder = []model.InvoiceType{
	model.InvoiceExportLUT,
	model.InvoiceExportIGST,
	model.InvoiceDomestic,
}

// InvoiceTypeLabel returns the human-readable display label for t, e.g.
// "Export (LUT)" for model.InvoiceExportLUT. An unrecognised type (a future
// enum value this build predates) falls back to its raw string rather than
// silently disappearing; a genuinely empty/zero type renders as "Unknown"
// so the gap is visible instead of rendering a blank cell.
func InvoiceTypeLabel(t model.InvoiceType) string {
	if lbl, ok := invoiceTypeLabels[t]; ok {
		return lbl
	}
	if t == "" {
		return "Unknown"
	}
	return string(t)
}

// InvoiceTypeFromLabel reverses InvoiceTypeLabel: given a display label as
// presented in a dropdown, it returns the underlying model.InvoiceType to
// store. ok is false when label doesn't match any known type's display
// label. Exported so any picker of human-readable options (e.g. the
// editor's type widget.Select) can be built on top of this same mapping
// without duplicating it.
func InvoiceTypeFromLabel(label string) (model.InvoiceType, bool) {
	for t, lbl := range invoiceTypeLabels {
		if lbl == label {
			return t, true
		}
	}
	return "", false
}

// InvoiceTypeLabels returns the display labels for every known invoice
// type, in a fixed order suitable for populating a widget.Select.
func InvoiceTypeLabels() []string {
	labels := make([]string, len(invoiceTypeOrder))
	for i, t := range invoiceTypeOrder {
		labels[i] = invoiceTypeLabels[t]
	}
	return labels
}

// invoiceTypeTone assigns each invoice type a badge tint, so the treatment an
// invoice was raised under is scannable in a list without reading the words.
// The tones are categorical, not a severity ramp — an IGST export is not
// "worse" than an LUT one, it is simply a different tax treatment.
func invoiceTypeTone(t model.InvoiceType) badgeTone {
	switch t {
	case model.InvoiceExportLUT:
		return toneInk
	case model.InvoiceExportIGST:
		return toneAmber
	case model.InvoiceDomestic:
		return toneGreen
	default:
		return toneNeutral
	}
}
