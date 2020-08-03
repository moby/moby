// +build !windows

package fs

import (
	"os"
	"syscall"
)

const (
	defaultRootDirMode = os.ModeDir | 0700
	defaultSymlinkMode = os.ModeSymlink | 0777
)

func newResourceFromInfo(info os.FileInfo) resource {
	statT := info.Sys().(*syscall.Stat_t)
	return resource{
		mode: info.Mode(),
		uid:  statT.Uid,
		gid:  statT.Gid,
	}
}

func (p *filePath) SetMode(mode os.FileMode) {
	p.file.mode = mode
}

func (p *directoryPath) SetMode(mode os.FileMode) {
	p.directory.mode = mode | os.ModeDir
}
