//go:build !windows
// +build !windows

package system

import (
	iofs "io/fs"
	"syscall"
	"time"

	"github.com/containerd/continuity/fs"
	"github.com/pkg/errors"
)

func Atime(st iofs.FileInfo) (time.Time, error) {
	stSys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, errors.Errorf("expected st.Sys() to be *syscall.Stat_t, got %T", st.Sys())
	}
	return fs.StatATimeAsTime(stSys), nil
}
