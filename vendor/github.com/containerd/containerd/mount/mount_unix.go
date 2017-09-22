// +build darwin freebsd

package mount

import "github.com/pkg/errors"

var (
	ErrNotImplementOnUnix = errors.New("not implemented under unix")
)

func (m *Mount) Mount(target string) error {
	return ErrNotImplementOnUnix
}

func Unmount(mount string, flags int) error {
	return ErrNotImplementOnUnix
}

func UnmountAll(mount string, flags int) error {
	return ErrNotImplementOnUnix
}
