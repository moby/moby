package container

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// compareMountPaths compares mount paths by depth so that parent paths sort
// before paths beneath them.
func compareMountPaths(a, b string) int {
	a = filepath.Clean(a)
	b = filepath.Clean(b)

	aParts := strings.Count(a, string(os.PathSeparator))
	bParts := strings.Count(b, string(os.PathSeparator))
	if c := cmp.Compare(aParts, bParts); c != 0 {
		return c
	}
	return cmp.Compare(a, b)
}

// SortMounts sorts mounts by destination depth so that parent mounts are
// applied before mounts beneath them. Mounts at the same depth are sorted
// by destination to produce a deterministic order.
//
// For example, /etc must be mounted before /etc/resolv.conf.
func SortMounts(m []Mount) {
	slices.SortStableFunc(m, func(a, b Mount) int {
		return compareMountPaths(a.Destination, b.Destination)
	})
}
