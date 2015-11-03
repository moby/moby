//+build !windows

package daemon

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	derr "github.com/docker/docker/errors"
)

// ContainerTop lists the processes running inside of the given
// container by calling ps with the given args, or with the flags
// "-ef" if no args are given.  An error is returned if the container
// is not found, or is not running, or if there are any problems
// running ps, or parsing the output.
func (daemon *Daemon) ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error) {
	if psArgs == "" {
		psArgs = "-ef"
	}

	container, err := daemon.Get(name)
	if err != nil {
		return nil, err
	}

	if !container.IsRunning() {
		return nil, derr.ErrorCodeNotRunning.WithArgs(name)
	}

	pids, err := daemon.ExecutionDriver().GetPidsForContainer(container.ID)
	if err != nil {
		return nil, err
	}

	output, err := exec.Command("ps", strings.Split(psArgs, " ")...).Output()
	if err != nil {
		return nil, derr.ErrorCodePSError.WithArgs(err)
	}

	procList := &types.ContainerProcessList{}

	lines := strings.Split(string(output), "\n")
	procList.Titles = strings.Fields(lines[0])

	pidIndex := -1
	for i, name := range procList.Titles {
		if name == "PID" {
			pidIndex = i
		}
	}
	if pidIndex == -1 {
		return nil, derr.ErrorCodeNoPID
	}

	// loop through the output and extract the PID from each line
	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}
		fields := strings.Fields(line)
		p, err := strconv.Atoi(fields[pidIndex])
		if err != nil {
			return nil, derr.ErrorCodeBadPID.WithArgs(fields[pidIndex], err)
		}

		for _, pid := range pids {
			if pid == p {
				// Make sure number of fields equals number of header titles
				// merging "overhanging" fields
				process := fields[:len(procList.Titles)-1]
				process = append(process, strings.Join(fields[len(procList.Titles)-1:], " "))
				procList.Processes = append(procList.Processes, process)
			}
		}
	}
	daemon.LogContainerEvent(container, "top")
	return procList, nil
}
