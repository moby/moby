package daemon

import (
	"github.com/moby/go-archive"
)

// defaultTarCopyOptions is the setting that is used when unpacking an archive
// for a copy API event.
func (daemon *Daemon) defaultTarCopyOptions(allowOverwriteDirWithFile bool) *archive.TarOptions {
	return &archive.TarOptions{
		NoOverwriteDirNonDir: !allowOverwriteDirWithFile,
		IDMap:                daemon.idMapping,
	}
}
