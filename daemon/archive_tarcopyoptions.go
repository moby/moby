package daemon

import (
	"github.com/docker/docker/pkg/archive"
)

// defaultTarCopyOptions is the setting that is used when unpacking an archive
// for a copy API event.
func (daemon *Daemon) defaultTarCopyOptions(noOverwriteDirNonDir bool) *archive.TarOptions {
	uidMaps, gidMaps := daemon.GetUIDGIDMaps()
	return &archive.TarOptions{
		NoOverwriteDirNonDir: noOverwriteDirNonDir,
		UIDMaps:              uidMaps,
		GIDMaps:              gidMaps,
	}
}
