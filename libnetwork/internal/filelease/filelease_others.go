//go:build !linux

package filelease

import (
	"os"

	"github.com/docker/docker/libnetwork/types"
)

func getWriteLease(f *os.File, opts *Opts) error {
	return types.NotImplementedErrorf("not implemented")
}
