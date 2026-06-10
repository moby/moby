//go:build windows

package fsutil

import (
	"time"

	"github.com/tonistiigi/fsutil/types"
)

func rewriteRootMetadata(root Root, p string, stat *types.Stat) error {
	return root.LChtimes(p, time.Unix(0, stat.ModTime))
}
