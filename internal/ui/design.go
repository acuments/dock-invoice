package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// This file is the design system: every spacing value, text size, surface and
// accent used anywhere in the UI resolves to something declared here.
//
// Two rules keep the app coherent, and both were broken before:
//
//  1. Hierarchy comes from type size and weight, never from colour. The accent
//     is spent only on actions. (Page titles used to be rendered with
//     widget.LowImportance/HighImportance, which in Fyne paints them in the
//     primary colour — so every screen title read as a small blue hyperlink.)
//  2. Vertical space is chosen, not inherited. widget.Label carries its own
//     inner padding, so stacking labels produced ~35px voids in some places
//     and 2px collisions in others. Captions and headings here are canvas.Text
//     (zero padding) and the gaps between them are explicit.

// ---------------------------------------------------------------------------
// Spacing scale — a 4pt grid. Use these, never a raw number.
// ---------------------------------------------------------------------------

const (
	spaceXs float32 = 4
	spaceSm float32 = 8
	spaceMd float32 = 12
	spaceLg float32 = 16
	spaceXl float32 = 24
)

// ---------------------------------------------------------------------------
// Type scale
// ---------------------------------------------------------------------------

const (
	textDisplay float32 = 26   // page title, one per screen
	textTitle   float32 = 18   // document title / dominant figure
	textBody    float32 = 13.5 // entered data, button labels
	textLabel   float32 = 12   // field captions
	textCaption float32 = 11   // hints, secondary meta
	textKicker  float32 = 10   // uppercase section kickers, badges
)

// ---------------------------------------------------------------------------
// Content widths
// ---------------------------------------------------------------------------

const (
	// editorFormMaxWidth caps the editor's form column. It is set by the
	// widest thing it has to hold — the nine-column line-item grid — rather
	// than by a prose reading measure, since every field on this page is a
	// short labelled input rather than running text.
	editorFormMaxWidth float32 = 880
	// summaryRailWidth is the editor's fixed right-hand totals rail.
	summaryRailWidth float32 = 320
	// settingsContentMaxWidth keeps Settings readable as a composed page.
	settingsContentMaxWidth float32 = 720
)

// ---------------------------------------------------------------------------
// Text primitives — all canvas.Text, so they carry no hidden padding
// ---------------------------------------------------------------------------

func text(s string, size float32, bold bool, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: bold}
	return t
}

// displayText is the single large heading at the top of a screen. Ink, not
// accent — it is a label for where you are, not something you can press.
func displayText(s string) *canvas.Text {
	return text(s, textDisplay, true, themeColor(theme.ColorNameForeground))
}

// titleText is a document-level heading ("Tax invoice") or a dominant figure.
func titleText(s string) *canvas.Text {
	return text(s, textTitle, true, themeColor(theme.ColorNameForeground))
}

// kickerText is the small uppercase rule-line above a group of fields. Muted
// and wide-set, it names a section without competing with the data inside it —
// the stationery equivalent of a printed box label.
func kickerText(s string) *canvas.Text {
	return text(strings.ToUpper(s), textKicker, true, themeColor(theme.ColorNamePlaceHolder))
}

// captionText labels a single field. Deliberately muted rather than bold ink:
// when every caption was bold black they out-shouted the values beneath them.
func captionText(s string) *canvas.Text {
	return text(s, textLabel, false, themeColor(theme.ColorNamePlaceHolder))
}

// hintText is quiet supporting copy (format examples, consequences).
func hintText(s string) *canvas.Text {
	return text(s, textCaption, false, themeColor(theme.ColorNameDisabled))
}

// mutedBodyText is running copy that should recede (page subtitles).
func mutedBodyText(s string) *canvas.Text {
	return text(s, textBody, false, themeColor(theme.ColorNamePlaceHolder))
}

// amountText renders a money figure right-aligned. The embedded Noto Sans has
// no monospace companion, so columns of figures are aligned by anchoring them
// to a shared right edge rather than by relying on tabular numerals.
func amountText(s string, size float32, bold bool, c color.Color) *canvas.Text {
	t := text(s, size, bold, c)
	t.Alignment = fyne.TextAlignTrailing
	return t
}

// ---------------------------------------------------------------------------
// Structure primitives
// ---------------------------------------------------------------------------

// gap is explicit vertical space between stacked blocks.
func gap(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}

// hgap is explicit horizontal space.
func hgap(w float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, 1))
	return r
}

// rule is a hairline divider in the separator colour.
func rule() fyne.CanvasObject {
	r := canvas.NewRectangle(themeColor(theme.ColorNameSeparator))
	r.SetMinSize(fyne.NewSize(1, 1))
	return r
}

// inkRule is the heavier rule printed above a total on a real invoice.
func inkRule() fyne.CanvasObject {
	r := canvas.NewRectangle(themeColor(theme.ColorNameForeground))
	r.SetMinSize(fyne.NewSize(1, 1))
	return r
}

// pad insets content by a uniform amount.
func pad(amount float32, content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(amount, amount, amount, amount), content)
}

// padXY insets content with separate vertical and horizontal amounts.
func padXY(v, h float32, content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(v, v, h, h), content)
}

// stack lays children in a column with a chosen, uniform gap. Fyne's VBox
// applies theme padding between every child, which is the wrong unit at most
// of the sizes this design uses.
func stack(spacing float32, items ...fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedVBoxLayout(spacing), items...)
}

// row lays children horizontally with a chosen gap, each at its natural width.
func row(spacing float32, items ...fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedHBoxLayout(spacing), items...)
}

// ---------------------------------------------------------------------------
// Surfaces
// ---------------------------------------------------------------------------

// sheetRadius is the corner radius of every sheet, and the radius anything
// docked to a sheet's edge has to follow to sit inside it.
const sheetRadius float32 = 10

// sheet is the primary surface: a white page floating on the warm canvas,
// defined by a hairline rather than a shadow.
func sheet(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(themeColor(theme.ColorNameInputBackground))
	bg.StrokeColor = themeColor(theme.ColorNameInputBorder)
	bg.StrokeWidth = 1
	bg.CornerRadius = sheetRadius
	return container.NewStack(bg, content)
}

// tintedSheet is a sheet on the warm header wash — used where a block should
// read as attached chrome (summary rail, table header) rather than a page.
func tintedSheet(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(themeColor(theme.ColorNameHeaderBackground))
	bg.StrokeColor = themeColor(theme.ColorNameInputBorder)
	bg.StrokeWidth = 1
	bg.CornerRadius = sheetRadius
	return container.NewStack(bg, content)
}

// bandFill is the warm wash behind a band docked to the top or bottom edge of
// a sheet (a table's column headers, a table's totals row).
//
// A plain rectangle will not do, which is what the invoice table's header used
// to use: painted corner to corner it covered both the sheet's rounded corners
// and the hairline border tracing them, so the sheet ended up with two square,
// unbordered edges and a radius that only survived where no band was docked.
// The wash is therefore inset by the border's own width and drawn rounded,
// with a square stripe laid over the inner edge — the edge that meets the
// sheet's content and must stay flat.
func bandFill(edge bandEdge) fyne.CanvasObject {
	tint := themeColor(theme.ColorNameHeaderBackground)

	rounded := canvas.NewRectangle(tint)
	rounded.CornerRadius = sheetRadius - 1
	stripe := canvas.NewRectangle(tint)
	stripe.SetMinSize(fyne.NewSize(1, sheetRadius))

	var flatten *fyne.Container
	if edge == bandTop {
		flatten = container.NewBorder(nil, stripe, nil, nil)
	} else {
		flatten = container.NewBorder(stripe, nil, nil, nil)
	}
	return pad(1, container.NewStack(rounded, flatten))
}

// bandEdge names which edge of a sheet a bandFill is docked to.
type bandEdge int

const (
	bandTop bandEdge = iota
	bandBottom
)

// fieldSection is the standard grouping inside a sheet: an uppercase kicker,
// a hairline, then the fields. The kicker/rule/body rhythm is fixed here so
// every section on every screen breathes identically.
func fieldSection(title string, body fyne.CanvasObject) fyne.CanvasObject {
	return fieldSectionWith(title, "", nil, body)
}

// fieldSectionWith adds an optional hint beside the kicker and an optional
// control docked at the right of the kicker line (e.g. a "saved customer"
// picker).
func fieldSectionWith(title, hint string, trailing, body fyne.CanvasObject) fyne.CanvasObject {
	var head fyne.CanvasObject = kickerText(title)
	if strings.TrimSpace(hint) != "" {
		head = row(spaceSm, head, hintText(hint))
	}
	if trailing != nil {
		head = container.New(&baselineTrailingLayout{}, head, trailing)
	}
	return stack(spaceSm, head, rule(), gap(spaceXs), body)
}

// wrappedHint is quiet supporting copy that must wrap. canvas.Text cannot
// wrap, so anything whose length depends on prose (section blurbs, zero-data
// messages) uses a Label here while short, fixed strings stay on canvas.Text.
func wrappedHint(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Importance = widget.LowImportance
	l.Wrapping = fyne.TextWrapWord
	return l
}

// tight cancels the inner padding widget.Label bakes into its own MinSize, so
// a Label can sit in a stack() and obey the spacing scale like every canvas
// primitive does. Without it a one-line blurb reserves an extra ~20px of
// height and sits ~10px right of the heading it belongs to, which is what
// reopened the ragged gaps this design system exists to close.
func tight(o fyne.CanvasObject) fyne.CanvasObject {
	p := themeSize(theme.SizeNameInnerPadding)
	return container.New(layout.NewCustomPaddedLayout(-p, -p, -p, -p), o)
}

// tabStripGutter horizontally insets a Fyne tab strip so its labels sit on the
// page gutter like everything else on the screen.
//
// container.AppTabs draws its buttons hard against its own left edge, inset
// only by the tab button's inner padding, so a nested tab row lands ~14px to
// the left of the page title above it — three different left edges on one
// screen. The compensation is expressed against the theme value that actually
// produces the inset rather than as a bare number, so it tracks the theme.
func tabStripGutter(tabs fyne.CanvasObject) fyne.CanvasObject {
	inset := spaceXl - themeSize(theme.SizeNameInnerPadding)
	if inset < 0 {
		inset = 0
	}
	return padXY(0, inset, tabs)
}

// settingsSection is one titled block on a Settings page: a sheet carrying a
// heading, a line explaining what the fields affect, and the fields.
func settingsSection(title, blurb string, body fyne.CanvasObject) fyne.CanvasObject {
	head := stack(spaceXs, text(title, textBody, true, themeColor(theme.ColorNameForeground)))
	if strings.TrimSpace(blurb) != "" {
		head.Add(tight(wrappedHint(blurb)))
	}
	return sheet(pad(spaceXl, stack(spaceLg, head, rule(), body)))
}

// ---------------------------------------------------------------------------
// Fields
// ---------------------------------------------------------------------------

// field is caption-above-control, the primary form pattern.
func field(caption string, w fyne.CanvasObject) fyne.CanvasObject {
	return stack(spaceXs, captionText(caption), w)
}

// fieldWithHint stacks caption, control, and a quiet explanatory line.
func fieldWithHint(caption string, w fyne.CanvasObject, hint string) fyne.CanvasObject {
	if strings.TrimSpace(hint) == "" {
		return field(caption, w)
	}
	return stack(spaceXs, captionText(caption), w, hintText(hint))
}

// fields lays labelled fields in one horizontal strip with relative weights.
func fields(weights []float32, items ...fyne.CanvasObject) fyne.CanvasObject {
	if len(weights) != len(items) {
		weights = make([]float32, len(items))
		for i := range weights {
			weights[i] = 1
		}
	}
	return container.New(&ratioHBox{weights: weights, gap: spaceMd}, items...)
}

// ---------------------------------------------------------------------------
// Badges
// ---------------------------------------------------------------------------

// badgeTone is a tinted pill's colour pair, resolved per theme variant.
type badgeTone struct {
	lightBG, lightFG color.Color
	darkBG, darkFG   color.Color
}

var (
	toneInk = badgeTone{
		lightBG: color.NRGBA{0xe8, 0xed, 0xf4, 0xff}, lightFG: color.NRGBA{0x1f, 0x3a, 0x5f, 0xff},
		darkBG: color.NRGBA{0x23, 0x2c, 0x38, 0xff}, darkFG: color.NRGBA{0x9c, 0xbd, 0xe6, 0xff},
	}
	toneAmber = badgeTone{
		lightBG: color.NRGBA{0xf7, 0xed, 0xda, 0xff}, lightFG: color.NRGBA{0x7a, 0x53, 0x10, 0xff},
		darkBG: color.NRGBA{0x33, 0x2c, 0x1c, 0xff}, darkFG: color.NRGBA{0xd9, 0xa5, 0x3f, 0xff},
	}
	toneGreen = badgeTone{
		lightBG: color.NRGBA{0xe6, 0xef, 0xe8, 0xff}, lightFG: color.NRGBA{0x2e, 0x6b, 0x47, 0xff},
		darkBG: color.NRGBA{0x1e, 0x2b, 0x23, 0xff}, darkFG: color.NRGBA{0x63, 0xb0, 0x84, 0xff},
	}
	toneNeutral = badgeTone{
		lightBG: color.NRGBA{0xef, 0xeb, 0xe3, 0xff}, lightFG: color.NRGBA{0x6b, 0x65, 0x59, 0xff},
		darkBG: color.NRGBA{0x2a, 0x28, 0x25, 0xff}, darkFG: color.NRGBA{0x97, 0x90, 0x7f, 0xff},
	}
)

func (b badgeTone) colors() (bg, fg color.Color) {
	if currentVariantIsDark() {
		return b.darkBG, b.darkFG
	}
	return b.lightBG, b.lightFG
}

// currentVariantIsDark reports whether the app is running the dark palette.
// It asks the same source appTheme does (INVOICER_THEME) rather than the Fyne
// app's own ThemeVariant: appTheme deliberately ignores the variant Fyne
// passes it, so on a machine set to dark mode with INVOICER_THEME unset, the
// app variant says "dark" while every colour on screen is the light palette.
// Reading the app variant here would tint badges for a theme that is not in
// use — light pills on a light sheet, or dark pills on a dark one.
func currentVariantIsDark() bool {
	return themeVariantFromEnv() == theme.VariantDark
}

// badge is a small tinted pill used to make a categorical value (invoice type)
// scannable in a list without spending the accent colour on it.
func badge(label string, tone badgeTone) fyne.CanvasObject {
	bgCol, fgCol := tone.colors()
	bg := canvas.NewRectangle(bgCol)
	bg.CornerRadius = 4
	t := text(strings.ToUpper(label), textKicker, true, fgCol)
	return container.NewStack(bg, padXY(3, spaceSm, t))
}

// ---------------------------------------------------------------------------
// Page chrome
// ---------------------------------------------------------------------------

// pageHeader is the standard screen masthead: display title, one quiet line of
// support copy, and optional trailing actions — on a tight, deliberate rhythm.
func pageHeader(title, subtitle string, trailing fyne.CanvasObject) fyne.CanvasObject {
	head := stack(spaceXs, displayText(title))
	if strings.TrimSpace(subtitle) != "" {
		head.Add(mutedBodyText(subtitle))
	}
	var content fyne.CanvasObject = head
	if trailing != nil {
		content = container.NewBorder(nil, nil, nil, trailing, head)
	}
	return padXY(spaceXl, spaceXl, content)
}

// emptyState centres a calm zero-data message. The width cap matters: without
// it, Fyne wraps the body copy to whatever narrow column the parent offers and
// the message collapses into an unreadable ribbon.
func emptyState(title, body string, cta fyne.CanvasObject) fyne.CanvasObject {
	t := text(title, textBody, true, themeColor(theme.ColorNameForeground))
	t.Alignment = fyne.TextAlignCenter
	// The body is a wrapping Label, not canvas.Text: zero-data copy is a full
	// sentence, and inside a narrow pane a non-wrapping line would spill past
	// both edges of the panel it is centred in.
	b := wrappedHint(body)
	b.Alignment = fyne.TextAlignCenter

	col := stack(spaceSm, t, tight(b))
	if cta != nil {
		col.Add(gap(spaceXs))
		col.Add(container.NewCenter(cta))
	}
	return container.NewCenter(container.New(&fixedWidthLayout{w: 320}, col))
}

// ---------------------------------------------------------------------------
// Layouts
// ---------------------------------------------------------------------------

// baselineTrailingLayout puts a title on the left, bottom-aligned with a
// taller trailing control on the right, so a tall picker never floats the
// section title up into dead space.
type baselineTrailingLayout struct{}

func (l *baselineTrailingLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	title, trail := objects[0], objects[1]
	tms, trms := title.MinSize(), trail.MinSize()
	trail.Resize(trms)
	trail.Move(fyne.NewPos(size.Width-trms.Width, (size.Height-trms.Height)/2))
	title.Resize(tms)
	title.Move(fyne.NewPos(0, (size.Height-tms.Height)/2))
}

func (l *baselineTrailingLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		if len(objects) == 1 && objects[0] != nil {
			return objects[0].MinSize()
		}
		return fyne.NewSize(0, 0)
	}
	tms, trms := objects[0].MinSize(), objects[1].MinSize()
	h := fyne.Max(tms.Height, trms.Height)
	return fyne.NewSize(tms.Width+spaceMd+trms.Width, h)
}

type fixedWidthLayout struct{ w float32 }

func (l *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	o := objects[0]
	h := fyne.Max(o.MinSize().Height, size.Height)
	o.Move(fyne.NewPos(0, 0))
	o.Resize(fyne.NewSize(l.w, h))
}

func (l *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objects {
		if o == nil {
			continue
		}
		h = fyne.Max(h, o.MinSize().Height)
	}
	return fyne.NewSize(l.w, h)
}

// maxWidthLayout constrains content to a maximum width and centres it.
type maxWidthLayout struct{ max float32 }

func (l *maxWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	o := objects[0]
	w := fyne.Min(size.Width, l.max)
	if w < 0 {
		w = 0
	}
	x := (size.Width - w) / 2
	if x < 0 {
		x = 0
	}
	h := fyne.Max(o.MinSize().Height, size.Height)
	o.Move(fyne.NewPos(x, 0))
	o.Resize(fyne.NewSize(w, h))
}

func (l *maxWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	ms := objects[0].MinSize()
	return fyne.NewSize(fyne.Min(ms.Width, l.max), ms.Height)
}

// ratioHBox packs children horizontally with relative column weights.
type ratioHBox struct {
	weights []float32
	gap     float32
}

// weightFor returns the weight of column i, defaulting to 1 when the caller
// supplied fewer weights than children.
func (r *ratioHBox) weightFor(i int) float32 {
	if i < len(r.weights) {
		return r.weights[i]
	}
	return 1
}

func (r *ratioHBox) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	g := r.gap
	if g <= 0 {
		g = spaceMd
	}

	// Hidden children are skipped entirely — they take neither width nor a
	// gap. Fields in a strip are shown and hidden per invoice type, and
	// laying a hidden one out anyway would leave a hole in the row and hand
	// the visible fields the wrong share of the width.
	var totalW float32
	visible := 0
	for i, o := range objects {
		if o == nil || !o.Visible() {
			continue
		}
		totalW += r.weightFor(i)
		visible++
	}
	if visible == 0 || totalW <= 0 {
		return
	}

	avail := size.Width - g*float32(visible-1)
	if avail < 0 {
		avail = 0
	}
	x := float32(0)
	for i, o := range objects {
		if o == nil || !o.Visible() {
			continue
		}
		w := avail * (r.weightFor(i) / totalW)
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(w, size.Height))
		x += w + g
	}
}

func (r *ratioHBox) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h, w float32
	g := r.gap
	if g <= 0 {
		g = spaceMd
	}
	visible := 0
	for _, o := range objects {
		if o == nil || !o.Visible() {
			continue
		}
		ms := o.MinSize()
		h = fyne.Max(h, ms.Height)
		w += ms.Width
		if visible > 0 {
			w += g
		}
		visible++
	}
	return fyne.NewSize(w, h)
}

// vCenterLayout gives its child the full available width but only its natural
// height, centred vertically. Table cells need exactly this: a canvas.Text
// must span the cell to honour trailing alignment, while still sitting on the
// cell's optical centre line rather than its top edge.
type vCenterLayout struct{}

func (vCenterLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		if o == nil {
			continue
		}
		h := o.MinSize().Height
		o.Resize(fyne.NewSize(size.Width, h))
		o.Move(fyne.NewPos(0, (size.Height-h)/2))
	}
}

func (vCenterLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		if o == nil {
			continue
		}
		ms := o.MinSize()
		w = fyne.Max(w, ms.Width)
		h = fyne.Max(h, ms.Height)
	}
	return fyne.NewSize(w, h)
}

// vcenter vertically centres content while letting it fill the width.
func vcenter(content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(vCenterLayout{}, content)
}

// maxWidthCenter constrains content to maxW and centres it.
func maxWidthCenter(maxW float32, content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&maxWidthLayout{max: maxW}, content)
}

// fixedWidth forces an object's layout width.
func fixedWidth(w float32, obj fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&fixedWidthLayout{w: w}, obj)
}

// ---------------------------------------------------------------------------
// Theme access
// ---------------------------------------------------------------------------

// themeColor resolves a theme color via the running app when available.
func themeColor(name fyne.ThemeColorName) color.Color {
	if a := fyne.CurrentApp(); a != nil {
		if th := a.Settings().Theme(); th != nil {
			return th.Color(name, a.Settings().ThemeVariant())
		}
	}
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

// themeSize resolves a theme size via the running app when available.
func themeSize(name fyne.ThemeSizeName) float32 {
	if a := fyne.CurrentApp(); a != nil {
		if th := a.Settings().Theme(); th != nil {
			return th.Size(name)
		}
	}
	return theme.DefaultTheme().Size(name)
}
