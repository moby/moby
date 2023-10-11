//go:build !linux

package vfs // import "github.com/docker/docker/daemon/graphdriver/vfs"

import (
	"github.com/containerd/continuity/fs"
)

func dirCopy(srcDir, dstDir string) error {
	return fs.CopyDir(dstDir, srcDir)
}
