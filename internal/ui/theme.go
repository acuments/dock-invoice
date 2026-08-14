package ui

import (
	"image/color"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"dock-invoice/internal/pdf"
)

// appTheme dresses the app as a piece of stationery rather than a SaaS
// dashboard: a warm paper canvas, pure white "sheets" floating on it, ink-black
// text, and a single deep ink-blue reserved exclusively for actions. Nothing
// here is bright — the hierarchy is carried by type size and weight, with
// hairlines instead of shadows.
//
// The accent is never used for headings or wayfinding. A blue word on this
// palette means "you can do something here", so spending it on a page title
// (as the previous theme did) both flattened the hierarchy and made titles
// read as hyperlinks.
//
// Default is light. Set INVOICER_THEME=dark for the warm-charcoal palette.
// Fallbacks still go through theme.DefaultTheme so unknown names stay safe.
type appTheme struct {
	regular fyne.Resource
	bold    fyne.Resource
	variant fyne.ThemeVariant
}

// NewTheme builds the application Fyne theme (light by default).
func NewTheme() fyne.Theme {
	return &appTheme{
		regular: fyne.NewStaticResource("NotoSans-Regular.ttf", pdf.NotoSansRegularBytes()),
		bold:    fyne.NewStaticResource("NotoSans-Bold.ttf", pdf.NotoSansBoldBytes()),
		variant: themeVariantFromEnv(),
	}
}

func themeVariantFromEnv() fyne.ThemeVariant {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("INVOICER_THEME"))) {
	case "dark":
		return theme.VariantDark
	default:
		return theme.VariantLight
	}
}

func (t *appTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Bold {
		return t.bold
	}
	return t.regular
}

func (t *appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *appTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return spaceSm
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius:
		// Crisper than a pill: this is a form on paper, not a chat bubble.
		return 7
	case theme.SizeNameSelectionRadius:
		return 5
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameText:
		return textBody
	case theme.SizeNameHeadingText:
		return textDisplay
	case theme.SizeNameSubHeadingText:
		return textTitle
	case theme.SizeNameCaptionText:
		return textCaption
	case theme.SizeNameSeparatorThickness:
		return 1
	default:
		return theme.DefaultTheme().Size(name)
	}
}

func (t *appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	_ = variant
	v := t.variant
	if v == theme.VariantDark {
		if c, ok := darkPalette(name); ok {
			return c
		}
	} else {
		if c, ok := lightPalette(name); ok {
			return c
		}
	}
	if c, ok := accentColor(name, v); ok {
		return c
	}
	return theme.DefaultTheme().Color(name, v)
}

// ---------------------------------------------------------------------------
// Light — warm paper
// ---------------------------------------------------------------------------

func lightPalette(name fyne.ThemeColorName) (color.Color, bool) {
	switch name {
	case theme.ColorNameBackground:
		// Warm off-white canvas (#FAF8F4) — the desk the sheets lie on.
		return rgb(0xfa, 0xf8, 0xf4), true
	case theme.ColorNameButton:
		return rgb(0xf2, 0xee, 0xe7), true
	case theme.ColorNameDisabledButton:
		return rgb(0xed, 0xe9, 0xe1), true
	case theme.ColorNameDisabled:
		return rgb(0xa9, 0xa2, 0x96), true
	case theme.ColorNameError:
		// Warm brick, not a fire-engine red — it has to sit on paper.
		return rgb(0xa6, 0x39, 0x2c), true
	case theme.ColorNameForeground:
		// Ink (#1A1A18), very slightly warm of pure black.
		return rgb(0x1a, 0x1a, 0x18), true
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return rgb(0xff, 0xff, 0xff), true
	case theme.ColorNameHover:
		return rgba(0x1a, 0x1a, 0x18, 0x0d), true
	case theme.ColorNameHeaderBackground:
		return rgb(0xf4, 0xf0, 0xe8), true
	case theme.ColorNameInputBackground:
		// The sheet itself stays pure white so entered data reads crisply.
		return rgb(0xff, 0xff, 0xff), true
	case theme.ColorNameInputBorder:
		return rgb(0xda, 0xd3, 0xc6), true
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return rgb(0xff, 0xff, 0xff), true
	case theme.ColorNamePlaceHolder:
		return rgb(0x7a, 0x75, 0x68), true
	case theme.ColorNamePressed:
		return rgba(0x1a, 0x1a, 0x18, 0x18), true
	case theme.ColorNameScrollBar:
		return rgba(0x1a, 0x1a, 0x18, 0x28), true
	case theme.ColorNameScrollBarBackground:
		return rgba(0x00, 0x00, 0x00, 0x00), true
	case theme.ColorNameSeparator:
		return rgb(0xe7, 0xe1, 0xd6), true
	case theme.ColorNameShadow:
		// Warm shadow — a cool grey shadow on warm paper reads as dirt.
		return rgba(0x3c, 0x32, 0x23, 0x1f), true
	case theme.ColorNameSuccess:
		return rgb(0x2e, 0x6b, 0x47), true
	case theme.ColorNameWarning:
		return rgb(0x96, 0x69, 0x0f), true
	case theme.ColorNameInnerWindowBorder:
		return rgb(0xda, 0xd3, 0xc6), true
	case theme.ColorNameInnerWindowBorderInactive:
		return rgb(0xe7, 0xe1, 0xd6), true
	default:
		return nil, false
	}
}

// ---------------------------------------------------------------------------
// Dark — warm charcoal (INVOICER_THEME=dark)
// ---------------------------------------------------------------------------

func darkPalette(name fyne.ThemeColorName) (color.Color, bool) {
	switch name {
	case theme.ColorNameBackground:
		return rgb(0x17, 0x16, 0x14), true
	case theme.ColorNameButton:
		return rgb(0x2a, 0x28, 0x25), true
	case theme.ColorNameDisabledButton:
		return rgb(0x22, 0x21, 0x1e), true
	case theme.ColorNameDisabled:
		return rgb(0x6d, 0x68, 0x5d), true
	case theme.ColorNameError:
		return rgb(0xe0, 0x7a, 0x6b), true
	case theme.ColorNameForeground:
		return rgb(0xed, 0xe9, 0xe1), true
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return rgb(0xff, 0xff, 0xff), true
	case theme.ColorNameHover:
		return rgba(0xff, 0xff, 0xff, 0x0f), true
	case theme.ColorNameHeaderBackground:
		return rgb(0x20, 0x1e, 0x1b), true
	case theme.ColorNameInputBackground:
		return rgb(0x1f, 0x1e, 0x1b), true
	case theme.ColorNameInputBorder:
		return rgb(0x3a, 0x37, 0x33), true
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return rgb(0x1f, 0x1e, 0x1b), true
	case theme.ColorNamePlaceHolder:
		return rgb(0x97, 0x90, 0x7f), true
	case theme.ColorNamePressed:
		return rgba(0xff, 0xff, 0xff, 0x18), true
	case theme.ColorNameScrollBar:
		return rgba(0xff, 0xff, 0xff, 0x28), true
	case theme.ColorNameScrollBarBackground:
		return rgba(0x00, 0x00, 0x00, 0x00), true
	case theme.ColorNameSeparator:
		return rgb(0x2e, 0x2c, 0x28), true
	case theme.ColorNameShadow:
		return rgba(0x00, 0x00, 0x00, 0x70), true
	case theme.ColorNameSuccess:
		return rgb(0x63, 0xb0, 0x84), true
	case theme.ColorNameWarning:
		return rgb(0xd9, 0xa5, 0x3f), true
	case theme.ColorNameInnerWindowBorder:
		return rgb(0x3a, 0x37, 0x33), true
	case theme.ColorNameInnerWindowBorderInactive:
		return rgb(0x2e, 0x2c, 0x28), true
	default:
		return nil, false
	}
}

// accentColor — deep ink blue, the colour of a good fountain pen. Reserved for
// actions, focus, and selection; never for headings (see the type note above).
func accentColor(name fyne.ThemeColorName, variant fyne.ThemeVariant) (color.Color, bool) {
	primary := rgb(0x1f, 0x3a, 0x5f) // #1F3A5F
	onPrimary := rgb(0xff, 0xff, 0xff)
	if variant == theme.VariantDark {
		// The dark accent has to be light enough to carry on a near-black
		// ground, which makes it far too light to sit white text on: white on
		// #7FA8D9 is about 2:1. Ink on the same fill is ~7.6:1.
		primary = rgb(0x7f, 0xa8, 0xd9)
		onPrimary = rgb(0x14, 0x18, 0x1e)
	}

	switch name {
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return primary, true
	case theme.ColorNameForegroundOnPrimary:
		return onPrimary, true
	case theme.ColorNameFocus:
		if variant == theme.VariantLight {
			return rgba(0x1f, 0x3a, 0x5f, 0x33), true
		}
		return rgba(0x7f, 0xa8, 0xd9, 0x4d), true
	case theme.ColorNameSelection:
		if variant == theme.VariantLight {
			return rgba(0x1f, 0x3a, 0x5f, 0x14), true
		}
		return rgba(0x7f, 0xa8, 0xd9, 0x2b), true
	default:
		return nil, false
	}
}

// flushListTheme is the app theme with the two sizes that hold a widget.List's
// rows apart zeroed out, for a table whose rows should meet edge to edge.
//
// widget.List spaces every row by SizeNamePadding and rounds each row's
// hover/selection background by SizeNameSelectionRadius. HideSeparators does
// not help: it hides the drawn separator line but keeps the gap that line sat
// in, so the rows still floated 8px apart with the sheet showing through, and
// the highlight read as a rounded pill laid on a row rather than as the row
// itself lighting up.
//
// Both sizes are load-bearing everywhere else — SizeNamePadding is the whole
// app's 8pt rhythm — so they are overridden only for the subtree the list
// occupies, via container.NewThemeOverride, never on the app theme.
type flushListTheme struct{ fyne.Theme }

func (t flushListTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding, theme.SizeNameSelectionRadius:
		return 0
	}
	return t.Theme.Size(name)
}

// flushList wraps a list so its rows sit flush. The app theme is resolved once,
// here, rather than tracked: this app picks its variant from INVOICER_THEME at
// startup and never swaps theme afterwards.
func flushList(list fyne.CanvasObject) fyne.CanvasObject {
	base := theme.DefaultTheme()
	if a := fyne.CurrentApp(); a != nil {
		if th := a.Settings().Theme(); th != nil {
			base = th
		}
	}
	return container.NewThemeOverride(list, flushListTheme{Theme: base})
}

func rgb(r, g, b uint8) color.Color {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

func rgba(r, g, b, a uint8) color.Color {
	return color.NRGBA{R: r, G: g, B: b, A: a}
}
