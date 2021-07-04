package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"io"

	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/system"
)

// ContainerExport writes the contents of the container to the given
// writer. An error is returned if the container cannot be found.
func (daemon *Daemon) ContainerExport(name string, out io.Writer) error {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	if isWindows && ctr.OS == "windows" {
		return fmt.Errorf("the daemon on this operating system does not support exporting Windows containers")
	}

	if ctr.IsDead() {
		err := fmt.Errorf("You cannot export container %s which is Dead", ctr.ID)
		return errdefs.Conflict(err)
	}

	if ctr.IsRemovalInProgress() {
		err := fmt.Errorf("You cannot export container %s which is being removed", ctr.ID)
		return errdefs.Conflict(err)
	}

	data, err := daemon.containerExport(ctr)
	if err != nil {
		return fmt.Errorf("Error exporting container %s: %v", name, err)
	}
	defer data.Close()

	// Stream the entire contents of the container (basically a volatile snapshot)
	if _, err := io.Copy(out, data); err != nil {
		return fmt.Errorf("Error exporting container %s: %v", name, err)
	}
	return nil
}

func (daemon *Daemon) containerExport(container *container.Container) (arch io.ReadCloser, err error) {
	if !system.IsOSSupported(container.OS) {
		return nil, fmt.Errorf("cannot export %s: %s ", container.ID, system.ErrNotSupportedOperatingSystem)
	}
	rwlayer, err := daemon.imageService.GetLayerByID(container.ID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			daemon.imageService.ReleaseLayer(rwlayer, container.OS)
		}
	}()

	basefs, err := rwlayer.Mount(container.GetMountLabel())
	if err != nil {
		return nil, err
	}

	archv, err := archivePath(basefs, basefs.Path(), &archive.TarOptions{
		Compression: archive.Uncompressed,
		UIDMaps:     daemon.idMapping.UIDs(),
		GIDMaps:     daemon.idMapping.GIDs(),
	}, basefs.Path())
	if err != nil {
		rwlayer.Unmount()
		return nil, err
	}
	arch = ioutils.NewReadCloserWrapper(archv, func() error {
		err := archv.Close()
		rwlayer.Unmount()
		daemon.imageService.ReleaseLayer(rwlayer, container.OS)
		return err
	})
	daemon.LogContainerEvent(container, "export")
	return arch, err
}
