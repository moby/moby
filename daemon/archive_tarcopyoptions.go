package daemon // import "github.com/moby/moby/daemon"

import (
	"github.com/moby/moby/pkg/archive"
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
