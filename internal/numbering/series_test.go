package numbering

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestFY(t *testing.T) {
	cases := []struct {
		d    time.Time
		want string
	}{
		{date(2024, time.April, 30), "2425"},
		{date(2024, time.April, 1), "2425"}, // rollover boundary: on-or-after 1 April is the new FY
		{date(2024, time.March, 31), "2324"},
		{date(2025, time.March, 31), "2425"},
		{date(2025, time.April, 1), "2526"}, // next rollover
		{date(2024, time.December, 25), "2425"},
	}
	for _, c := range cases {
		if got := FY(c.d); got != c.want {
			t.Errorf("FY(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestExpand_Golden(t *testing.T) {
	got := Expand("AEX{FY}-{SEQ}", date(2024, time.April, 30), 1)
	want := "AEX2425-1"
	if got != want {
		t.Errorf("Expand = %q, want %q", got, want)
	}
}

func TestExpand_MultipleSequences(t *testing.T) {
	got := Expand("AEX{FY}-{SEQ}", date(2024, time.April, 30), 12)
	want := "AEX2425-12"
	if got != want {
		t.Errorf("Expand = %q, want %q", got, want)
	}
}

func TestSeriesKey_DiffersAcrossFY(t *testing.T) {
	k1 := SeriesKey("AEX{FY}-{SEQ}", date(2024, time.March, 31))
	k2 := SeriesKey("AEX{FY}-{SEQ}", date(2024, time.April, 1))
	if k1 == k2 {
		t.Errorf("expected different series keys across FY rollover, got same: %q", k1)
	}
}
