package remote // import "github.com/docker/docker/libcontainerd/remote"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/windows/hcsshimtypes"
	libcontainerdtypes "github.com/docker/docker/libcontainerd/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

const runtimeName = "io.containerd.runhcs.v1"

func summaryFromInterface(i interface{}) (*libcontainerdtypes.Summary, error) {
	switch pd := i.(type) {
	case *hcsshimtypes.ProcessDetails:
		return &libcontainerdtypes.Summary{
			CreateTimestamp:              pd.CreatedAt,
			ImageName:                    pd.ImageName,
			KernelTime100ns:              pd.KernelTime_100Ns,
			MemoryCommitBytes:            pd.MemoryCommitBytes,
			MemoryWorkingSetPrivateBytes: pd.MemoryWorkingSetPrivateBytes,
			MemoryWorkingSetSharedBytes:  pd.MemoryWorkingSetSharedBytes,
			ProcessId:                    pd.ProcessID,
			UserTime100ns:                pd.UserTime_100Ns,
		}, nil
	default:
		return nil, errors.Errorf("Unknown process details type %T", pd)
	}
}

func prepareBundleDir(bundleDir string, ociSpec *specs.Spec) (string, error) {
	// TODO: (containerd) Determine if we need to use system.MkdirAllWithACL here
	return bundleDir, os.MkdirAll(bundleDir, 0755)
}

func pipeName(containerID, processID, name string) string {
	return fmt.Sprintf(`\\.\pipe\containerd-%s-%s-%s`, containerID, processID, name)
}

func newFIFOSet(bundleDir, processID string, withStdin, withTerminal bool) *cio.FIFOSet {
	containerID := filepath.Base(bundleDir)
	config := cio.Config{
		Terminal: withTerminal,
		Stdout:   pipeName(containerID, processID, "stdout"),
	}

	if withStdin {
		config.Stdin = pipeName(containerID, processID, "stdin")
	}

	if !config.Terminal {
		config.Stderr = pipeName(containerID, processID, "stderr")
	}

	return cio.NewFIFOSet(config, nil)
}

func (c *client) newDirectIO(ctx context.Context, fifos *cio.FIFOSet) (*cio.DirectIO, error) {
	pipes, err := c.newStdioPipes(fifos)
	if err != nil {
		return nil, err
	}
	return cio.NewDirectIOFromFIFOSet(ctx, pipes.stdin, pipes.stdout, pipes.stderr, fifos), nil
}

func (c *client) UpdateResources(ctx context.Context, containerID string, resources *libcontainerdtypes.Resources) error {
	// TODO: (containerd): Not implemented, but don't error.
	return nil
}

func getSpecUser(ociSpec *specs.Spec) (int, int) {
	// TODO: (containerd): Not implemented, but don't error.
	// Not clear if we can even do this for LCOW.
	return 0, 0
}
