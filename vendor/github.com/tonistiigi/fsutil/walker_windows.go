// +build windows

package fsutil

import (
	"os"
)

func loadXattr(_ string, _ *Stat) error {
	return nil
}

func setUnixOpt(_ os.FileInfo, _ *Stat, _ string, _ map[uint64]string) {
}
