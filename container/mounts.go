package container

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type mounts []Mount

// Len returns the number of mounts. Used in sorting.
func (m mounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m mounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m mounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m mounts) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Destination), string(os.PathSeparator))
}

// SortMounts sorts an array of mounts in lexicographic order. This
// ensures that, when mounting, the mounts don't shadow other
// mounts. For example, if mounting /etc and /etc/resolv.conf,
// /etc/resolv.conf must not be mounted first.
func SortMounts(m []Mount) []Mount {
	sort.Sort(mounts(m))
	return m
}
