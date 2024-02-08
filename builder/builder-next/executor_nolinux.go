//go:build !linux

package buildkit

import (
	"context"
	"errors"
	"runtime"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	resourcetypes "github.com/moby/buildkit/executor/resources/types"
)

func newExecutor(_, _ string, _ *libnetwork.Controller, _ *oci.DNSConfig, _ bool, _ idtools.IdentityMapping, _ string) (executor.Executor, error) {
	return &stubExecutor{}, nil
}

type stubExecutor struct{}

func (w *stubExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (resourcetypes.Recorder, error) {
	return nil, errors.New("buildkit executor not implemented for " + runtime.GOOS)
}

func (w *stubExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) error {
	return errors.New("buildkit executor not implemented for " + runtime.GOOS)
}

func getDNSConfig(config.DNSConfig) *oci.DNSConfig {
	return nil
}
