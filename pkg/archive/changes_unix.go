// +build !windows

package archive

import (
	"os"
	"syscall"

	"github.com/docker/docker/pkg/system"
)

func statDifferent(oldStat *system.StatT, newStat *system.StatT) bool {
	// Don't look at size for dirs, its not a good measure of change
	if oldStat.Mode() != newStat.Mode() ||
		oldStat.UID() != newStat.UID() ||
		oldStat.GID() != newStat.GID() ||
		oldStat.Rdev() != newStat.Rdev() ||
		// Don't look at size for dirs, its not a good measure of change
		(oldStat.Mode()&syscall.S_IFDIR != syscall.S_IFDIR &&
			(!sameFsTimeSpec(oldStat.Mtim(), newStat.Mtim()) || (oldStat.Size() != newStat.Size()))) {
		return true
	}
	return false
}

func (info *FileInfo) isDir() bool {
	return info.parent == nil || info.stat.Mode()&syscall.S_IFDIR != 0
}

func GetInode(fi os.FileInfo) (inode uint64) {
	s, _ := fi.Sys().(*syscall.Stat_t)
	inode = uint64(s.Ino)
	return
}

func IsHardlink(fi os.FileInfo) bool {
	switch sys := fi.Sys().(type) {
	case *syscall.Stat_t:
		if fi.Mode().IsRegular() && sys.Nlink > 1 {
			return true
		}
	}
	return false
}
