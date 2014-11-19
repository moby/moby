// +build !linux,!osx,!freebsd,!openbsd

package archive

import "os"

func IsHardlink(fi os.FileInfo) bool {
	return false
}
