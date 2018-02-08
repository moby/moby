package zfs // import "github.com/docker/docker/daemon/graphdriver/zfs"

import (
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/sirupsen/logrus"
)

func checkRootdirFs(rootDir string) error {
	fsMagic, err := graphdriver.GetFSMagic(rootDir)
	if err != nil {
		return err
	}
	backingFS := "unknown"
	if fsName, ok := graphdriver.FsNames[fsMagic]; ok {
		backingFS = fsName
	}

	if fsMagic != graphdriver.FsMagicZfs {
		logrus.WithField("root", rootDir).WithField("backingFS", backingFS).WithField("driver", "zfs").Error("No zfs dataset found for root")
		return graphdriver.ErrPrerequisites
	}

	return nil
}

func getMountpoint(id string) string {
	return id
}
