package daemon

import (
	"io"

	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
)

// ContainerExport writes the contents of the container to the given
// writer. An error is returned if the container cannot be found.
func (daemon *Daemon) ContainerExport(name string, out io.Writer) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	data, err := daemon.containerExport(container)
	if err != nil {
		return derr.ErrorCodeExportFailed.WithArgs(name, err)
	}
	defer data.Close()

	// Stream the entire contents of the container (basically a volatile snapshot)
	if _, err := io.Copy(out, data); err != nil {
		return derr.ErrorCodeExportFailed.WithArgs(name, err)
	}
	return nil
}

func (daemon *Daemon) containerExport(container *Container) (archive.Archive, error) {
	if err := daemon.Mount(container); err != nil {
		return nil, err
	}

	uidMaps, gidMaps := daemon.GetUIDGIDMaps()
	archive, err := archive.TarWithOptions(container.basefs, &archive.TarOptions{
		Compression: archive.Uncompressed,
		UIDMaps:     uidMaps,
		GIDMaps:     gidMaps,
	})
	if err != nil {
		daemon.Unmount(container)
		return nil, err
	}
	arch := ioutils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		daemon.Unmount(container)
		return err
	})
	daemon.LogContainerEvent(container, "export")
	return arch, err
}
