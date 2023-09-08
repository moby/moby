package versions // import "github.com/docker/docker/api/types/versions"

import (
	"strconv"
	"strings"
)

// compare compares two version strings
// returns -1 if v1 < v2, 1 if v1 > v2, 0 otherwise.
func compare(v1, v2 string) int {
	if v1 == v2 {
		return 0
	}
	var (
		currTab  = strings.Split(v1, ".")
		otherTab = strings.Split(v2, ".")
	)

	maxVer := len(currTab)
	if len(otherTab) > maxVer {
		maxVer = len(otherTab)
	}
	for i := 0; i < maxVer; i++ {
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
	return 0
}

// LessThan checks if a version is less than another
func LessThan(v, other string) bool {
	return compare(v, other) == -1
}

// LessThanOrEqualTo checks if a version is less than or equal to another
func LessThanOrEqualTo(v, other string) bool {
	return compare(v, other) <= 0
}

// GreaterThan checks if a version is greater than another
func GreaterThan(v, other string) bool {
	return compare(v, other) == 1
}

// GreaterThanOrEqualTo checks if a version is greater than or equal to another
func GreaterThanOrEqualTo(v, other string) bool {
	return compare(v, other) >= 0
}

// Equal checks if a version is equal to another
func Equal(v, other string) bool {
	return compare(v, other) == 0
}
