//go:build windows
// +build windows

package contenthash

import (
	"os"

	fstypes "github.com/tonistiigi/fsutil/types"
)

func setUnixOpt(_ string, _ os.FileInfo, _ *fstypes.Stat) error {
	return nil
}
