package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
)

// ContainerExport writes the contents of the container to the given
// writer. An error is returned if the container cannot be found.
func (daemon *Daemon) ContainerExport(ctx context.Context, name string, out io.Writer) error {
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

	err = daemon.containerExport(ctx, ctr, out)
	if err != nil {
		return fmt.Errorf("Error exporting container %s: %v", name, err)
	}

	return nil
}

func (daemon *Daemon) containerExport(ctx context.Context, container *container.Container, out io.Writer) error {
	err := daemon.imageService.PerformWithBaseFS(ctx, container, func(basefs string) error {
		archv, err := chrootarchive.Tar(basefs, &archive.TarOptions{
			Compression: archive.Uncompressed,
			IDMap:       daemon.idMapping,
		}, basefs)
		if err != nil {
			return err
		}

		// Stream the entire contents of the container (basically a volatile snapshot)
		_, err = io.Copy(out, archv)
		return err
	})
	if err != nil {
		return err
	}
	daemon.LogContainerEvent(container, "export")
	return nil
}
