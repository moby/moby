// +build windows

package contenthash

import (
	"os"

	fstypes "github.com/tonistiigi/fsutil/types"
)

func setUnixOpt(path string, fi os.FileInfo, stat *fstypes.Stat) error {
	return nil
}
