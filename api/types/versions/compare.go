package versions // import "github.com/docker/docker/api/types/versions"

import (
	"strconv"
	"strings"
)

// compare compares two version strings
// returns -1 if v1 < v2, 1 if v1 > v2, 0 otherwise.
func compare(v1, v2 string) int {
	var (
		currTab         = strings.Split(trimSuffix(v1), ".")
		otherTab        = strings.Split(trimSuffix(v2), ".")
		curPreRelease   = trimSuffix(v1) != v1
		otherPreRelease = trimSuffix(v2) != v2
	)

	max := len(currTab)
	if len(otherTab) > max {
		max = len(otherTab)
	}
	for i := 0; i < max; i++ {
		var currInt, otherInt int

		if len(currTab) > i {
			currInt, _ = strconv.Atoi(currTab[i])
		}
		if len(otherTab) > i {
			otherInt, _ = strconv.Atoi(otherTab[i])
		}
		if currInt > otherInt {
			return 1
		}
		if otherInt > currInt {
			return -1
		}
	}

	// TODO this does not yet compare pre-release version (e.g. -beta.0 vs -beta.1, or -alpha.0 vs -beta.1)
	if curPreRelease == otherPreRelease {
		return 0
	}
	if curPreRelease {
		return -1
	}
	return 1
}

func trimSuffix(input string) string {
	parts := strings.SplitN(input, "-", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return input
}

// LessThan checks if a version is less than another. It is designed to compare
// API versions, but has limited support for SemVer(ish) versions
func LessThan(v, other string) bool {
	return compare(v, other) == -1
}

// LessThanOrEqualTo checks if a version is less than or equal to another. It is
// designed to compare API versions, but has limited support for SemVer(ish) versions
func LessThanOrEqualTo(v, other string) bool {
	return compare(v, other) <= 0
}

// GreaterThan checks if a version is greater than another. It is designed to
// compare API versions, but has limited support for SemVer(ish) versions
func GreaterThan(v, other string) bool {
	return compare(v, other) == 1
}

// GreaterThanOrEqualTo checks if a version is greater than or equal to another.
// It is designed to compare API versions, but has limited support for SemVer(ish)
// versions
func GreaterThanOrEqualTo(v, other string) bool {
	return compare(v, other) >= 0
}

// Equal checks if a version is equal to another. It is designed to compare API
// versions, but has limited support for SemVer(ish) versions
func Equal(v, other string) bool {
	return compare(v, other) == 0
}
