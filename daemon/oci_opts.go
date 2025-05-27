package daemon

import (
	"context"

	"github.com/containerd/containerd/v2/core/containers"
	coci "github.com/containerd/containerd/v2/pkg/oci"
	"github.com/docker/docker/container"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// WithConsoleSize sets the initial console size
func WithConsoleSize(c *container.Container) coci.SpecOpts {
	return func(ctx context.Context, _ coci.Client, _ *containers.Container, s *coci.Spec) error {
		if c.HostConfig.ConsoleSize[0] > 0 || c.HostConfig.ConsoleSize[1] > 0 {
			if s.Process == nil {
				s.Process = &specs.Process{}
			}
			s.Process.ConsoleSize = &specs.Box{
				Height: c.HostConfig.ConsoleSize[0],
				Width:  c.HostConfig.ConsoleSize[1],
			}
		}
		return nil
	}
}
