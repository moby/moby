// +build !linux,!windows

package system

import (
	"syscall"
)

// fromStatT creates a system.Stat_t type from a syscall.Stat_t type
func fromStatT(s *syscall.Stat_t) (*Stat_t, error) {
	return &Stat_t{size: s.Size,
		mode: uint32(s.Mode),
		uid:  s.Uid,
		gid:  s.Gid,
		rdev: uint64(s.Rdev),
		dev:  uint64(s.Dev),
		ino:  uint64(s.Ino),
		mtim: s.Mtimespec}, nil
}
