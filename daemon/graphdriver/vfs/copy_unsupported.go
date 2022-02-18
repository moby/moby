//go:build !linux
// +build !linux

package vfs // import "github.com/moby/moby/daemon/graphdriver/vfs"

import "github.com/moby/moby/pkg/chrootarchive"

func dirCopy(srcDir, dstDir string) error {
	return chrootarchive.NewArchiver(nil).CopyWithTar(srcDir, dstDir)
}
