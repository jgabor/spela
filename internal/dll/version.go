package dll

import (
	"strconv"
	"strings"
)

// CompareVersions compares two version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Versions are expected in the format "major.minor.patch" (e.g., "3.7.20").
func CompareVersions(a, b string) int {
	partsA := parseVersion(a)
	partsB := parseVersion(b)

	maxLen := max(len(partsA), len(partsB))

	for i := range maxLen {
		var va, vb int
		if i < len(partsA) {
			va = partsA[i]
		}
		if i < len(partsB) {
			vb = partsB[i]
		}

		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}

	return 0
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	result := make([]int, len(parts))

	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			result[i] = 0
		} else {
			result[i] = n
		}
	}

	return result
}

// IsNewer returns true if available is newer than installed.
func IsNewer(installed, available string) bool {
	return CompareVersions(installed, available) < 0
}
