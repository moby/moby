package graphdriver

import (
	_ "github.com/docker/docker/daemon/graphdriver/vfs"

	// TODO Windows - Add references to real graph driver when PR'd
)

type DiffDiskDriver interface {
	Driver
	CopyDiff(id, sourceId string) error
}

var (
	// Slice of drivers that should be used in order
	priority = []string{
		"windows",
		"vfs",
	}
)

func GetFSMagic(rootpath string) (FsMagic, error) {
	// Note it is OK to return FsMagicUnsupported on Windows.
	return FsMagicUnsupported, nil
}
