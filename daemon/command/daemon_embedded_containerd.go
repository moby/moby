//go:build !no_embedded_containerd

package command

import (
	"context"
	"path/filepath"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/containerd/server/embedded"
	"github.com/pkg/errors"
)

// initEmbeddedContainerd starts containerd inside the daemon process and points
// the containerd clients at it. It is selected by the "embedded-containerd"
// feature.
func (cli *daemonCLI) initEmbeddedContainerd(ctx context.Context) (func(time.Duration) error, error) {
	log.G(ctx).Warn("Running with experimental embedded-containerd mode")
	d, err := embedded.Start(
		ctx,
		filepath.Join(cli.Config.Root, "containerd"),
		filepath.Join(cli.Config.ExecRoot, "containerd"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start embedded containerd")
	}
	cli.Config.ContainerdAddr = d.Address()
	cli.containerdDialer = d.Dial

	return d.WaitTimeout, nil
}
