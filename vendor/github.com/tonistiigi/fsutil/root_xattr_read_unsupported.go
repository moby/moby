//go:build !linux && !darwin && !freebsd && !netbsd

package fsutil

import "github.com/tonistiigi/fsutil/types"

func loadRootXattr(Root, string, *types.Stat) error {
	return nil
}
