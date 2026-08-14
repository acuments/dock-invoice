// Package numbering expands invoice number patterns (e.g. "AEX{FY}-{SEQ}")
// against an Indian financial year (1 April - 31 March) and an
// auto-incrementing per-series sequence counter.
package numbering

import (
	"fmt"
	"strings"
	"time"
)

// FY computes the two-digit-pair financial year label for a date, e.g.
// 30 April 2024 -> "2425" (FY 2024-25), and 15 March 2024 -> "2324"
// (FY 2023-24, since the Indian FY runs 1 April - 31 March).
func FY(t time.Time) string {
	y := t.Year()
	if t.Month() < time.April {
		y--
	}
	start := y % 100
	end := (y + 1) % 100
	return fmt.Sprintf("%02d%02d", start, end)
}

// Expand renders a number pattern, replacing "{FY}" with the financial year
// label for date and "{SEQ}" with seq (no padding). Additional tokens can be
// added here later without changing callers.
func Expand(pattern string, date time.Time, seq int64) string {
	s := pattern
	s = strings.ReplaceAll(s, "{FY}", FY(date))
	s = strings.ReplaceAll(s, "{SEQ}", fmt.Sprintf("%d", seq))
	return s
}

// SeriesKey identifies an independent counter: one per (pattern, financial
// year) pair, so numbering restarts at 1 each FY within the same pattern
// while different invoice types/patterns never share a counter.
func SeriesKey(pattern string, date time.Time) string {
	return pattern + "|" + FY(date)
}
