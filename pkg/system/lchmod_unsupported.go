// +build !freebsd

package system

import (
	"os"
)

func LChmod(path string, mode os.FileMode) error {
	// There is no lchmod(2) on Linux, also OS X just has lchmod(3),
	// not lchmod(2).
	return nil
}
