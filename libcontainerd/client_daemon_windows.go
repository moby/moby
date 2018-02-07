package libcontainerd // import "github.com/docker/docker/libcontainerd"

import (
	"fmt"
	"path/filepath"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/windows/hcsshimtypes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func summaryFromInterface(i interface{}) (*Summary, error) {
	switch pd := i.(type) {
	case *hcsshimtypes.ProcessDetails:
		return &Summary{
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
	return bundleDir, nil
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
