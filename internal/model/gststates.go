package model

import (
	"sort"
	"strings"
)

// gstStateNames maps two-digit GST state/UT codes to their official names.
// Sourced from the CBIC/GST state-code list (codes 01–38 for states and UTs,
// plus special reporting codes 97 and 99). Legacy codes 25 and 28 are kept
// for GSTINs issued before the 2020 UT merger and the Andhra Pradesh
// bifurcation.
var gstStateNames = map[string]string{
	"01": "Jammu & Kashmir",
	"02": "Himachal Pradesh",
	"03": "Punjab",
	"04": "Chandigarh",
	"05": "Uttarakhand",
	"06": "Haryana",
	"07": "Delhi",
	"08": "Rajasthan",
	"09": "Uttar Pradesh",
	"10": "Bihar",
	"11": "Sikkim",
	"12": "Arunachal Pradesh",
	"13": "Nagaland",
	"14": "Manipur",
	"15": "Mizoram",
	"16": "Tripura",
	"17": "Meghalaya",
	"18": "Assam",
	"19": "West Bengal",
	"20": "Jharkhand",
	"21": "Odisha",
	"22": "Chhattisgarh",
	"23": "Madhya Pradesh",
	"24": "Gujarat",
	"25": "Daman & Diu", // legacy; merged into 26
	"26": "Dadra & Nagar Haveli and Daman & Diu",
	"27": "Maharashtra",
	"28": "Andhra Pradesh", // legacy; pre-2014 bifurcation
	"29": "Karnataka",
	"30": "Goa",
	"31": "Lakshadweep",
	"32": "Kerala",
	"33": "Tamil Nadu",
	"34": "Puducherry",
	"35": "Andaman & Nicobar Islands",
	"36": "Telangana",
	"37": "Andhra Pradesh",
	"38": "Ladakh",
	"97": "Other Territory",
	"99": "Centre Jurisdiction",
}

// normalizeGSTStateCode canonicalises a GST state code for map lookup: trims
// whitespace, strips redundant leading zeros, and zero-pads single-digit
// codes ("7" -> "07"). Returns "" for empty or all-zero input.
func normalizeGSTStateCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	trimmed := strings.TrimLeft(code, "0")
	if trimmed == "" {
		return ""
	}
	if len(trimmed) == 1 {
		return "0" + trimmed
	}
	return trimmed
}

// GSTStateName returns the official state/UT name for a GST state code.
// ok is false when the code is not recognised.
func GSTStateName(code string) (name string, ok bool) {
	code = normalizeGSTStateCode(code)
	if code == "" {
		return "", false
	}
	name, ok = gstStateNames[code]
	return name, ok
}

// GSTStateLabel formats a code and name as "33-Tamil Nadu". When name is
// empty the table is consulted; unknown codes are returned bare.
func GSTStateLabel(code, name string) string {
	code = normalizeGSTStateCode(code)
	name = strings.TrimSpace(name)
	if name == "" && code != "" {
		if resolved, ok := GSTStateName(code); ok {
			name = resolved
		}
	}
	if code == "" {
		return name
	}
	if name == "" {
		return code
	}
	return code + "-" + name
}

// GSTStateLabels returns every known "code-name" label in code order, for
// searchable place-of-supply pickers.
func GSTStateLabels() []string {
	codes := make([]string, 0, len(gstStateNames))
	for code := range gstStateNames {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	labels := make([]string, len(codes))
	for i, code := range codes {
		labels[i] = GSTStateLabel(code, gstStateNames[code])
	}
	return labels
}

// FormatGSTState expands a bare code ("33") to "33-Tamil Nadu". Already-
// formatted or unrecognised values are returned unchanged.
func FormatGSTState(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if _, ok := GSTStateName(s); ok {
		return GSTStateLabel(s, "")
	}
	return s
}
