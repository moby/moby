package buildkit

import (
	"context"
	"errors"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
)

func newExecutor(_, _ string, _ *libnetwork.Controller, _ *oci.DNSConfig, _ bool, _ idtools.IdentityMapping, _ string) (executor.Executor, error) {
	return &winExecutor{}, nil
}

type winExecutor struct {
}

func (w *winExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (err error) {
	return errors.New("buildkit executor not implemented for windows")
}

func (w *winExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) error {
	return errors.New("buildkit executor not implemented for windows")
}

func getDNSConfig(config.DNSConfig) *oci.DNSConfig {
	return nil
}
