// +build !windows

package buildkit

import (
	"path/filepath"

	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/runcexecutor"
)

func newExecutor(root string) (executor.Executor, error) {
	return runcexecutor.New(runcexecutor.Opt{
		Root:              filepath.Join(root, "executor"),
		CommandCandidates: []string{"docker-runc", "runc"},
	})
}
