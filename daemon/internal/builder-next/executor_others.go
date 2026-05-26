//go:build !linux && !windows

package buildkit

import (
	"github.com/moby/buildkit/executor"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// networkName is unused on unsupported platforms but referenced by the shared
// bridgeProvider in executor.go.
const networkName = ""

func newExecutor(executorOpts) (executor.Executor, error) {
	return &stubExecutor{}, nil
}

func (iface *lnInterface) Set(*specs.Spec) error {
	<-iface.ready
	return iface.err
}
