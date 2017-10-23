package libcontainerd

import (
	"fmt"

	"github.com/containerd/containerd"
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

func newFIFOSet(bundleDir, containerID, processID string, withStdin, withTerminal bool) *containerd.FIFOSet {
	fifos := &containerd.FIFOSet{
		Terminal: withTerminal,
		Out:      pipeName(containerID, processID, "stdout"),
	}

	if withStdin {
		fifos.In = pipeName(containerID, processID, "stdin")
	}

	if !fifos.Terminal {
		fifos.Err = pipeName(containerID, processID, "stderr")
	}

	return fifos
}
