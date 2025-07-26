//go:build !linux

package buildkit

import (
	"context"
	"errors"
	"runtime"

	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	resourcetypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/solver/llbsolver/cdidevices"
	"github.com/moby/moby/v2/daemon/libnetwork"
	"github.com/moby/sys/user"
)

type stubExecutor struct{}

func (w *stubExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (resourcetypes.Recorder, error) {
	return nil, errors.New("buildkit executor not implemented for " + runtime.GOOS)
}

func (w *stubExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) error {
	return errors.New("buildkit executor not implemented for " + runtime.GOOS)
}

// function stub created for GraphDriver
func newExecutorGD(_, _ string, _ *libnetwork.Controller, _ *oci.DNSConfig, _ bool, _ user.IdentityMapping, _ string, _ *cdidevices.Manager, _, _ string) (executor.Executor, error) {
	return &stubExecutor{}, nil
}
