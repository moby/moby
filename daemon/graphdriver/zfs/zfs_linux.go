package zfs

import (
	"fmt"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	derr "github.com/docker/docker/errors"
)

func checkRootdirFs(rootdir string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		return fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}

	if graphdriver.FsMagic(buf.Type) != graphdriver.FsMagicZfs {
		logrus.Debugf("[zfs] no zfs dataset found for rootdir '%s'", rootdir)
		return derr.ErrorCodeGDErrPrereqs
	}

	return nil
}

func getMountpoint(id string) string {
	return id
}
