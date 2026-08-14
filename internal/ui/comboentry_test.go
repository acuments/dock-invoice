package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"dock-invoice/internal/model"
)

// TestMatchComboOptions_FiltersByLabelOrDetail is the core "type to filter
// among many items" behaviour the searchable saved-item picker exists for:
// a query must match either the description (Label) or the HSN/SAC/rate
// summary (Detail), case-insensitively, and a blank query matches
// everything.
func TestMatchComboOptions_FiltersByLabelOrDetail(t *testing.T) {
	opts := []comboOption{
		{Label: "Software Development Service", Detail: "998314 · $4,000.00"},
		{Label: "Cloud infrastructure management", Detail: "998315 · $850.00"},
		{Label: "Domestic support retainer", Detail: "998313 · ₹15,000.00"},
	}

	cases := []struct {
		name  string
		query string
		want  []int
	}{
		{"blank query matches all", "", []int{0, 1, 2}},
		{"matches label substring case-insensitively", "software", []int{0}},
		{"matches label substring mid-word", "CLOUD", []int{1}},
		{"matches HSN/SKU code in Detail", "998313", []int{2}},
		{"matches nothing", "zzz-no-such-item", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchComboOptions(opts, c.query, 0)
			if !intSlicesEqual(got, c.want) {
				t.Errorf("matchComboOptions(%q) = %v, want %v", c.query, got, c.want)
			}
		})
	}
}

// TestMatchComboOptions_RespectsLimit covers the cap that keeps a several-
// hundred-SKU catalogue's first (unfiltered) render fast and scannable.
func TestMatchComboOptions_RespectsLimit(t *testing.T) {
	opts := make([]comboOption, 200)
	for i := range opts {
		opts[i] = comboOption{Label: fmt.Sprintf("Item %d", i)}
	}
	got := matchComboOptions(opts, "", 40)
	if len(got) != 40 {
		t.Fatalf("matchComboOptions with limit=40 returned %d results, want 40", len(got))
	}
	if got[0] != 0 || got[39] != 39 {
		t.Errorf("matchComboOptions capped results = first %d last %d, want original order 0..39", got[0], got[39])
	}
}

func intSlicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestItemComboOptions_IncludesHSNInDetail confirms the searchable item
// picker's option list carries the HSN/SAC (the closest thing this app has
// to a SKU) in Detail, so matchComboOptions can find an item by that code
// even when the user doesn't remember its description.
func TestItemComboOptions_IncludesHSNInDetail(t *testing.T) {
	items := []model.Item{
		{Description: "Consulting", HSNSAC: "998314", DefaultRate: 15000, Currency: model.CurrencyUSD},
		{Description: "", HSNSAC: "", DefaultRate: 0, Currency: model.CurrencyINR},
	}
	opts := itemComboOptions(items)
	if len(opts) != 2 {
		t.Fatalf("itemComboOptions len = %d, want 2", len(opts))
	}
	if opts[0].Label != "Consulting" {
		t.Errorf("opts[0].Label = %q, want Consulting", opts[0].Label)
	}
	if !strings.Contains(opts[0].Detail, "998314") {
		t.Errorf("opts[0].Detail = %q, want it to contain the HSN 998314", opts[0].Detail)
	}
	if opts[1].Label != "(unnamed item)" {
		t.Errorf("opts[1].Label = %q, want the blank-description fallback", opts[1].Label)
	}
}

func TestSearchComboBox_ListBackgroundIsOpaque(t *testing.T) {
	test.NewTempApp(t)
	box := newSearchComboBox("pick", "", []comboOption{{Label: "07-Delhi"}}, nil)
	if box.listBG == nil {
		t.Fatal("list background rectangle is nil")
	}
	nrgba, ok := color.NRGBAModel.Convert(box.listBG.FillColor).(color.NRGBA)
	if !ok {
		t.Fatalf("could not convert FillColor %T to NRGBA", box.listBG.FillColor)
	}
	if nrgba.A != 0xff {
		t.Errorf("list background alpha = %d, want 255 (opaque)", nrgba.A)
	}
}

func TestSearchComboBox_MinSizeGrowsWhenListOpen(t *testing.T) {
	test.NewTempApp(t)
	box := newSearchComboBox("pick", "", []comboOption{
		{Label: "07-Delhi"},
		{Label: "33-Tamil Nadu"},
	}, nil)
	closed := box.MinSize()
	box.openList("d")
	open := box.MinSize()
	if open.Height <= closed.Height {
		t.Errorf("open MinSize.Height = %v, want taller than closed %v so the list is not a transparent overlay", open.Height, closed.Height)
	}
}
