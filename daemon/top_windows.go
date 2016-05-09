package daemon

import (
	"errors"
	"strconv"

	"github.com/docker/engine-api/types"
)

// ContainerTop is a minimal implementation on Windows currently.
// TODO Windows: This needs more work, but needs platform API support.
// All we can currently return (particularly in the case of Hyper-V containers)
// is a PID and the command.
func (daemon *Daemon) ContainerTop(containerID string, psArgs string) (*types.ContainerProcessList, error) {

	// It's really not an equivalent to linux 'ps' on Windows
	if psArgs != "" {
		return nil, errors.New("Windows does not support arguments to top")
	}

	s, err := daemon.containerd.Summary(containerID)
	if err != nil {
		return nil, err
	}

	procList := &types.ContainerProcessList{}

	for _, v := range s {
		procList.Titles = append(procList.Titles, strconv.Itoa(int(v.Pid))+" "+v.Command)
	}
	return procList, nil
}
