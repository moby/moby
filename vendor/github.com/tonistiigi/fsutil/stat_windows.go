//go:build windows
// +build windows

package fsutil

import (
	"os"

	"github.com/tonistiigi/fsutil/types"
)

func loadXattr(_ string, _ *types.Stat) error {
	return nil
}

func setUnixOpt(_ os.FileInfo, _ *types.Stat, _ string, _ map[uint64]string) {
}
