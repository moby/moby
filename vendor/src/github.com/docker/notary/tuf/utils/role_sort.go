package utils

import (
	"strings"
)

// RoleList is a list of roles
type RoleList []string

// Len returns the length of the list
func (r RoleList) Len() int {
	return len(r)
}

// Less returns true if the item at i should be sorted
// before the item at j. It's an unstable partial ordering
// based on the number of segments, separated by "/", in
// the role name
func (r RoleList) Less(i, j int) bool {
	segsI := strings.Split(r[i], "/")
	segsJ := strings.Split(r[j], "/")
	if len(segsI) == len(segsJ) {
		return r[i] < r[j]
	}
	return len(segsI) < len(segsJ)
}

// Swap the items at 2 locations in the list
func (r RoleList) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}
