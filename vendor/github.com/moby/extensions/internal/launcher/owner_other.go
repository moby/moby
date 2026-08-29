//go:build !unix && !windows

package launcher

import "io/fs"

func fileUID(fs.FileInfo) (int, bool) {
	return 0, false
}
