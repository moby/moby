package zfs // import "github.com/docker/docker/daemon/graphdriver/zfs"

import (
	"context"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/internal/fstype"
)

func checkRootdirFs(rootDir string) error {
	fsMagic, err := fstype.GetFSMagic(rootDir)
	if err != nil {
		return err
	}
	backingFS := "unknown"
	if fsName, ok := fstype.FsNames[fsMagic]; ok {
		backingFS = fsName
	}

	if fsMagic != fstype.FsMagicZfs {
		log.G(context.TODO()).WithField("root", rootDir).WithField("backingFS", backingFS).WithField("storage-driver", "zfs").Error("No zfs dataset found for root")
		return graphdriver.ErrPrerequisites
	}

	return nil
}

func getMountpoint(id string) string {
	return id
}
