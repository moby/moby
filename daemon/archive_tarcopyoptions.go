package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
)

// defaultTarCopyOptions is the setting that is used when unpacking an archive
// for a copy API event.
func (daemon *Daemon) defaultTarCopyOptions(noOverwriteDirNonDir bool) *archive.TarOptions {
	return &archive.TarOptions{
		NoOverwriteDirNonDir: noOverwriteDirNonDir,
		IDMap:                idtools.FromUserIdentityMapping(daemon.idMapping),
	}
}
