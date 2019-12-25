package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/pkg/archive"
)

// defaultTarCopyOptions is the setting that is used when unpacking an archive
// for a copy API event.
func (daemon *Daemon) defaultTarCopyOptions(noOverwriteDirNonDir bool) *archive.TarOptions {
	return &archive.TarOptions{
		NoOverwriteDirNonDir: noOverwriteDirNonDir,
		UIDMaps:              daemon.idMapping.UIDs(),
		GIDMaps:              daemon.idMapping.GIDs(),
	}
}
