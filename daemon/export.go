package daemon

import (
	"io"

	"github.com/docker/docker/context"
	derr "github.com/docker/docker/errors"
)

// ContainerExport writes the contents of the container to the given
// writer. An error is returned if the container cannot be found.
func (daemon *Daemon) ContainerExport(ctx context.Context, name string, out io.Writer) error {
	container, err := daemon.Get(ctx, name)
	if err != nil {
		return err
	}

	data, err := container.export(ctx)
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
