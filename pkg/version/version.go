package version

import (
	"strconv"
	"strings"
)

type Version string

func (me Version) compareTo(other Version) int {
	var (
		meTab    = strings.Split(string(me), ".")
		otherTab = strings.Split(string(other), ".")
	)
	for i, s := range meTab {
		var meInt, otherInt int
		meInt, _ = strconv.Atoi(s)
		if len(otherTab) > i {
			otherInt, _ = strconv.Atoi(otherTab[i])
		}
		if meInt > otherInt {
			return 1
		}
		if otherInt > meInt {
			return -1
		}
	}
	if len(otherTab) > len(meTab) {
		return -1
	}
	return 0
}

func (me Version) LessThan(other Version) bool {
	return me.compareTo(other) == -1
}

func (me Version) LessThanOrEqualTo(other Version) bool {
	return me.compareTo(other) <= 0
}

func (me Version) GreaterThan(other Version) bool {
	return me.compareTo(other) == 1
}

func (me Version) GreaterThanOrEqualTo(other Version) bool {
	return me.compareTo(other) >= 0
}

func (me Version) Equal(other Version) bool {
	return me.compareTo(other) == 0
}
