//go:build !linux

package vfs // import "github.com/docker/docker/daemon/graphdriver/vfs"

import (
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/go-archive/chrootarchive"
)

func dirCopy(srcDir, dstDir string) error {
	return chrootarchive.NewArchiver(idtools.IdentityMapping{}).CopyWithTar(srcDir, dstDir)
}
