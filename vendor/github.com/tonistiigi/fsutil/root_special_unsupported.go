//go:build !linux && !freebsd && !netbsd && !openbsd && !dragonfly

package fsutil

import (
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

func handleRootTarTypeBlockCharFifo(RootMknod, string, *types.Stat) error {
	return errors.New("not implemented")
}
