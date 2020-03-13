// +build !windows,!linux,!freebsd freebsd,!cgo

package mountinfo

import (
	"fmt"
	"runtime"
)

func parseMountTable(f FilterFunc) ([]*Info, error) {
	return nil, fmt.Errorf("mount.parseMountTable is not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)
}
