// +build windows

package contenthash

import (
	"os"

	"github.com/tonistiigi/fsutil"
)

// chmodWindowsTarEntry is used to adjust the file permissions used in tar
// header based on the platform the archival is done.
func chmodWindowsTarEntry(perm os.FileMode) os.FileMode {
	perm &= 0755
	// Add the x bit: make everything +x from windows
	perm |= 0111

	return perm
}

func setUnixOpt(path string, fi os.FileInfo, stat *fsutil.Stat) error {
	return nil
}
