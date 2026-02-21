package update

import (
	"fmt"
	"strconv"
	"strings"
)

// compareVersions compares two semver strings (without "v" prefix).
// Returns -1 if a < b, 0 if a == b, +1 if a > b.
func compareVersions(a, b string) (int, error) {
	aParts, err := parseVersion(a)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", a, err)
	}
	bParts, err := parseVersion(b)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", b, err)
	}

	// Compare segment by segment; missing segments treated as 0.
	maxLen := max(len(aParts), len(bParts))
	for i := range maxLen {
		av, bv := 0, 0
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		if av < bv {
			return -1, nil
		}
		if av > bv {
			return 1, nil
		}
	}
	return 0, nil
}

// parseVersion splits a version string on "." and parses each segment as int.
func parseVersion(v string) ([]int, error) {
	parts := strings.Split(v, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("segment %q: %w", p, err)
		}
		if n < 0 {
			return nil, fmt.Errorf("segment %q: negative number", p)
		}
		nums[i] = n
	}
	return nums, nil
}

// stripVPrefix removes a leading "v" or "V" from a version string.
func stripVPrefix(version string) string {
	return strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
}
