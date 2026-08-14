package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

func TestAppTheme_DefaultIsLight(t *testing.T) {
	t.Setenv("INVOICER_THEME", "")
	th := NewTheme()
	bg := th.Color(theme.ColorNameBackground, theme.VariantDark)
	fg := th.Color(theme.ColorNameForeground, theme.VariantDark)
	if sameNRGBA(bg, fg) {
		t.Fatalf("light foreground and background are identical: %#v", bg)
	}
	r, _, _, _ := bg.RGBA()
	if r < 0x8000 {
		t.Fatalf("default theme should be light, got background %#v", bg)
	}
	primary := th.Color(theme.ColorNamePrimary, theme.VariantLight)
	if n, ok := primary.(color.NRGBA); !ok || n.A == 0 {
		t.Fatalf("primary should be opaque NRGBA, got %#v", primary)
	}
}

func TestAppTheme_DarkEnvUsesDarkPalette(t *testing.T) {
	t.Setenv("INVOICER_THEME", "dark")
	th := NewTheme()
	bg := th.Color(theme.ColorNameBackground, theme.VariantLight)
	fg := th.Color(theme.ColorNameForeground, theme.VariantLight)
	if sameNRGBA(bg, fg) {
		t.Fatalf("dark foreground and background are identical: %#v", bg)
	}
	primary := th.Color(theme.ColorNamePrimary, theme.VariantDark)
	if n, ok := primary.(color.NRGBA); !ok || n.A == 0 {
		t.Fatalf("primary should be opaque NRGBA, got %#v", primary)
	}
	// Near-black background
	r, _, _, _ := bg.RGBA()
	if r > 0x4000 {
		t.Fatalf("expected dark background, got %#v", bg)
	}
}

func TestAppTheme_LightEnvUsesLightPalette(t *testing.T) {
	t.Setenv("INVOICER_THEME", "light")
	th := NewTheme()
	bg := th.Color(theme.ColorNameBackground, theme.VariantDark)
	r, _, _, _ := bg.RGBA()
	if r < 0x8000 {
		t.Fatalf("INVOICER_THEME=light should yield a light background, got %#v", bg)
	}
}

func TestAppTheme_UsesEmbeddedNoto(t *testing.T) {
	th := NewTheme()
	reg := th.Font(fyne.TextStyle{})
	if reg == nil || len(reg.Content()) < 1000 {
		t.Fatal("expected embedded Noto Sans regular font bytes")
	}
	bold := th.Font(fyne.TextStyle{Bold: true})
	if bold == nil || len(bold.Content()) < 1000 {
		t.Fatal("expected embedded Noto Sans bold font bytes")
	}
}

func sameNRGBA(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
