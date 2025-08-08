//go:build !linux && !windows

package buildkit

import (
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/solver/llbsolver/cdidevices"
	"github.com/moby/moby/v2/daemon/libnetwork"
	"github.com/moby/sys/user"
)

func newExecutor(_, _ string, _ *libnetwork.Controller, _ *oci.DNSConfig, _ bool, _ user.IdentityMapping, _ string, _ *cdidevices.Manager, _, _ string) (executor.Executor, error) {
	return &stubExecutor{}, nil
}
