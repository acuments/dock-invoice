package model

import "testing"

func TestGSTStateName(t *testing.T) {
	cases := []struct {
		code string
		want string
		ok   bool
	}{
		{"33", "Tamil Nadu", true},
		{"07", "Delhi", true},
		{"7", "Delhi", true},
		{"97", "Other Territory", true},
		{"38", "Ladakh", true},
		{"", "", false},
		{"00", "", false},
		{"88", "", false},
	}
	for _, tc := range cases {
		got, ok := GSTStateName(tc.code)
		if ok != tc.ok || got != tc.want {
			t.Errorf("GSTStateName(%q) = (%q, %v), want (%q, %v)", tc.code, got, ok, tc.want, tc.ok)
		}
	}
}

func TestGSTStateLabel(t *testing.T) {
	cases := []struct {
		code, name string
		want       string
	}{
		{"33", "Tamil Nadu", "33-Tamil Nadu"},
		{"33", "", "33-Tamil Nadu"},
		{"7", "", "07-Delhi"},
		{"97", "", "97-Other Territory"},
		{"", "Tamil Nadu", "Tamil Nadu"},
		{"88", "", "88"},
	}
	for _, tc := range cases {
		if got := GSTStateLabel(tc.code, tc.name); got != tc.want {
			t.Errorf("GSTStateLabel(%q, %q) = %q, want %q", tc.code, tc.name, got, tc.want)
		}
	}
}

func TestGSTStateLabels_OrderedAndComplete(t *testing.T) {
	labels := GSTStateLabels()
	if len(labels) != len(gstStateNames) {
		t.Fatalf("len(GSTStateLabels) = %d, want %d", len(labels), len(gstStateNames))
	}
	if labels[0] != "01-Jammu & Kashmir" {
		t.Errorf("first = %q, want 01-Jammu & Kashmir", labels[0])
	}
	if labels[len(labels)-1] != "99-Centre Jurisdiction" {
		t.Errorf("last = %q, want 99-Centre Jurisdiction", labels[len(labels)-1])
	}
	foundTN := false
	for _, l := range labels {
		if l == "33-Tamil Nadu" {
			foundTN = true
			break
		}
	}
	if !foundTN {
		t.Error("GSTStateLabels missing 33-Tamil Nadu")
	}
}

func TestFormatGSTState(t *testing.T) {
	if got := FormatGSTState("33"); got != "33-Tamil Nadu" {
		t.Errorf("FormatGSTState(33) = %q, want 33-Tamil Nadu", got)
	}
	if got := FormatGSTState("33-Tamil Nadu"); got != "33-Tamil Nadu" {
		t.Errorf("FormatGSTState(already labelled) = %q", got)
	}
}

func TestCompany_StateLabel_ResolvesNameFromCode(t *testing.T) {
	c := Company{StateCode: "33"}
	if got := c.StateLabel(); got != "33-Tamil Nadu" {
		t.Errorf("StateLabel() = %q, want %q", got, "33-Tamil Nadu")
	}
}
