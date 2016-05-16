package zfs

import (
	"fmt"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/storage"
)

func checkRootdirFs(rootdir string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		return fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}

	if storage.FsMagic(buf.Type) != storage.FsMagicZfs {
		logrus.Debugf("[zfs] no zfs dataset found for rootdir '%s'", rootdir)
		return storage.ErrPrerequisites
	}

	return nil
}

func getMountpoint(id string) string {
	return id
}
