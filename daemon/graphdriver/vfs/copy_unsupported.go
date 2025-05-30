//go:build !linux

package vfs

import (
	"github.com/moby/go-archive/chrootarchive"
	"github.com/moby/sys/user"
)

func dirCopy(srcDir, dstDir string) error {
	return chrootarchive.NewArchiver(user.IdentityMapping{}).CopyWithTar(srcDir, dstDir)
}
