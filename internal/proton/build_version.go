package proton

import (
	"regexp"
	"strconv"
)

var cachyOSBuildTagPattern = regexp.MustCompile(`(?i)cachyos[-_]?(\d+)\.(\d+)-(\d{8})`)

// CachyOSBuildTag is the MAJOR.MINOR-YYYYMMDD segment from a proton-cachyos
// Steam tool name (e.g. cachyos-11.0-20260521-slr).
type CachyOSBuildTag struct {
	Major int
	Minor int
	Date  int
}

// ParseCachyOSBuildTag extracts a proton-cachyos build tag from a compat-tool
// name. Returns false for non-CachyOS tools such as GE-Proton.
func ParseCachyOSBuildTag(name string) (CachyOSBuildTag, bool) {
	matches := cachyOSBuildTagPattern.FindStringSubmatch(name)
	if len(matches) != 4 {
		return CachyOSBuildTag{}, false
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return CachyOSBuildTag{}, false
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return CachyOSBuildTag{}, false
	}
	date, err := strconv.Atoi(matches[3])
	if err != nil {
		return CachyOSBuildTag{}, false
	}
	return CachyOSBuildTag{Major: major, Minor: minor, Date: date}, true
}

// Compare orders two build tags lexicographically by major, minor, then date.
func (t CachyOSBuildTag) Compare(other CachyOSBuildTag) int {
	if t.Major != other.Major {
		return t.Major - other.Major
	}
	if t.Minor != other.Minor {
		return t.Minor - other.Minor
	}
	return t.Date - other.Date
}

func parseMinimumBuildTag(raw string) (CachyOSBuildTag, bool) {
	return ParseCachyOSBuildTag("cachyos-" + raw + "-slr")
}

func buildTagMeetsMinimum(name, minimum string) bool {
	tag, ok := ParseCachyOSBuildTag(name)
	if !ok {
		return false
	}
	minTag, ok := parseMinimumBuildTag(minimum)
	if !ok {
		return false
	}
	return tag.Compare(minTag) >= 0
}
