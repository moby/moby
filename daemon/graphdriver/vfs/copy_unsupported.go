//go:build !linux
// +build !linux

package vfs // import "github.com/docker/docker/daemon/graphdriver/vfs"

import "github.com/docker/docker/pkg/chrootarchive"

func dirCopy(srcDir, dstDir string) error {
	return chrootarchive.NewArchiver(nil).CopyWithTar(srcDir, dstDir)
}
