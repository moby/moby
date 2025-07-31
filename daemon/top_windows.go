package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/container"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
)

// ContainerTop handles `docker top` client requests.
//
// Future considerations:
//   - Windows users are far more familiar with CPU% total.
//     Further, users on Windows rarely see user/kernel CPU stats split.
//     The kernel returns everything in terms of 100ns. To obtain
//     CPU%, we could do something like docker stats does which takes two
//     samples, subtract the difference and do the maths. Unfortunately this
//     would slow the stat call down and require two kernel calls. So instead,
//     we do something similar to linux and display the CPU as combined HH:MM:SS.mmm.
//   - Perhaps we could add an argument to display "raw" stats
//   - "Memory" is an extremely overloaded term in Windows. Hence we do what
//     task manager does and use the private working set as the memory counter.
//     We could return more info for those who really understand how memory
//     management works in Windows if we introduced a "raw" stats (above).
func (daemon *Daemon) ContainerTop(name string, psArgs string) (*container.TopResponse, error) {
	// It's not at all an equivalent to linux 'ps' on Windows
	if psArgs != "" {
		return nil, errors.New("Windows does not support arguments to top")
	}

	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	task, err := func() (libcontainerdtypes.Task, error) {
		ctr.Lock()
		defer ctr.Unlock()

		task, err := ctr.GetRunningTask()
		if err != nil {
			return nil, err
		}
		if ctr.Restarting {
			return nil, errContainerIsRestarting(ctr.ID)
		}
		return task, nil
	}()

	s, err := task.Summary(context.Background())
	if err != nil {
		return nil, err
	}
	procList := &container.TopResponse{
		Titles: []string{"Name", "PID", "CPU", "Private Working Set"},
	}

	for _, j := range s {
		d := time.Duration((j.KernelTime_100Ns + j.UserTime_100Ns) * 100) // Combined time in nanoseconds
		procList.Processes = append(procList.Processes, []string{
			j.ImageName,
			fmt.Sprint(j.ProcessID),
			fmt.Sprintf("%02d:%02d:%02d.%03d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60, int(d.Nanoseconds()/1000000)%1000),
			units.HumanSize(float64(j.MemoryWorkingSetPrivateBytes)),
		})
	}

	return procList, nil
}
