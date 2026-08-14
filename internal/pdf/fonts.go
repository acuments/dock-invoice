package pdf

import (
	_ "embed"

	"github.com/go-pdf/fpdf"
)

//go:embed assets/NotoSans-Regular.ttf
var notoSansRegular []byte

//go:embed assets/NotoSans-Bold.ttf
var notoSansBold []byte

// FontFamily is the family name registered for Noto Sans, used for every
// piece of text so that ₹ (U+20B9) renders correctly on every platform
// without depending on system fonts.
const FontFamily = "NotoSans"

// registerFonts loads the embedded Noto Sans Regular and Bold weights into
// the given document.
func registerFonts(pdf *fpdf.Fpdf) {
	pdf.AddUTF8FontFromBytes(FontFamily, "", notoSansRegular)
	pdf.AddUTF8FontFromBytes(FontFamily, "B", notoSansBold)
}

// NotoSansRegularBytes returns the embedded Noto Sans Regular TTF bytes.
// Exported so other packages (e.g. internal/ui, for the Fyne theme) can
// reuse the exact same font the PDF is rendered with, instead of embedding
// a second copy of the font file.
func NotoSansRegularBytes() []byte { return notoSansRegular }

// NotoSansBoldBytes returns the embedded Noto Sans Bold TTF bytes.
func NotoSansBoldBytes() []byte { return notoSansBold }
