package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/containerd/log"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/chrootarchive"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/errdefs"
)

// ContainerExport writes the contents of the container to the given
// writer. An error is returned if the container cannot be found.
func (daemon *Daemon) ContainerExport(ctx context.Context, name string, out io.Writer) error {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	if isWindows && ctr.ImagePlatform.OS == "windows" {
		return errors.New("the daemon on this operating system does not support exporting Windows containers")
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

func (daemon *Daemon) containerExport(ctx context.Context, ctr *container.Container, out io.Writer) error {
	rwl := ctr.RWLayer
	if rwl == nil {
		return fmt.Errorf("container %s has no rootfs", ctr.ID)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	basefs, err := rwl.Mount(ctr.GetMountLabel())
	if err != nil {
		return err
	}
	defer func() {
		if err := rwl.Unmount(); err != nil {
			log.G(ctx).WithFields(log.Fields{"error": err, "container": ctr.ID}).Warn("Failed to unmount container RWLayer after export")
		}
	}()

	archv, err := chrootarchive.Tar(basefs, &archive.TarOptions{
		Compression: archive.Uncompressed,
		IDMap:       daemon.idMapping,
	}, basefs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	context.AfterFunc(ctx, func() {
		_ = archv.Close()
	})

	// Stream the entire contents of the container (basically a volatile snapshot)
	if _, err := io.Copy(out, archv); err != nil {
		if err := ctx.Err(); err != nil {
			return errdefs.Cancelled(err)
		}
		return err
	}

	daemon.LogContainerEvent(ctr, events.ActionExport)
	return nil
}
