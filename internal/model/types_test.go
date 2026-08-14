package model

import "testing"

func TestNormalizeLayoutStyle(t *testing.T) {
	cases := []struct {
		name string
		in   LayoutStyle
		want LayoutStyle
	}{
		{"empty (pre-field rows) normalizes to modern", "", LayoutModern},
		{"modern stays modern", LayoutModern, LayoutModern},
		{"classic stays classic", LayoutClassic, LayoutClassic},
		{"unrecognised value falls back to modern", LayoutStyle("nonsense"), LayoutModern},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeLayoutStyle(tc.in); got != tc.want {
				t.Errorf("NormalizeLayoutStyle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEffectiveLayoutStyle(t *testing.T) {
	cases := []struct {
		name     string
		invoice  LayoutStyle
		settings LayoutStyle
		want     LayoutStyle
	}{
		{"empty invoice follows classic settings", "", LayoutClassic, LayoutClassic},
		{"empty invoice follows modern settings", "", LayoutModern, LayoutModern},
		{"explicit classic beats modern settings", LayoutClassic, LayoutModern, LayoutClassic},
		{"explicit modern beats classic settings", LayoutModern, LayoutClassic, LayoutModern},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveLayoutStyle(tc.invoice, tc.settings); got != tc.want {
				t.Errorf("EffectiveLayoutStyle(%q, %q) = %q, want %q", tc.invoice, tc.settings, got, tc.want)
			}
		})
	}
}

func TestLayoutStyleLabel_And_FromLabel_RoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		style LayoutStyle
		label string
	}{
		{"modern", LayoutModern, "Modern (colour)"},
		{"classic", LayoutClassic, "Classic (black & white)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LayoutStyleLabel(tc.style); got != tc.label {
				t.Errorf("LayoutStyleLabel(%q) = %q, want %q", tc.style, got, tc.label)
			}
			gotStyle, ok := LayoutStyleFromLabel(tc.label)
			if !ok {
				t.Fatalf("LayoutStyleFromLabel(%q) ok = false, want true", tc.label)
			}
			if gotStyle != tc.style {
				t.Errorf("LayoutStyleFromLabel(%q) = %q, want %q", tc.label, gotStyle, tc.style)
			}
		})
	}

	t.Run("unknown value normalizes to modern label", func(t *testing.T) {
		if got := LayoutStyleLabel(LayoutStyle("nonsense")); got != "Modern (colour)" {
			t.Errorf("LayoutStyleLabel(nonsense) = %q, want %q", got, "Modern (colour)")
		}
	})

	t.Run("unrecognised label reports ok=false", func(t *testing.T) {
		if _, ok := LayoutStyleFromLabel("Sepia"); ok {
			t.Error("LayoutStyleFromLabel(\"Sepia\") ok = true, want false")
		}
	})
}
