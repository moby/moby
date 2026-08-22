package daemon

import (
	"testing"
	"time"

	"github.com/moby/moby/v2/daemon/container"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type monitorMockTask struct {
	libcontainerdtypes.Task
	pid uint32
}

func (t *monitorMockTask) Pid() uint32 {
	return t.pid
}

func TestShouldIgnoreExitEventWithLockForRunningContainer(t *testing.T) {
	previousExit := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	currentStart := previousExit.Add(time.Second)
	currentExit := currentStart.Add(time.Second)

	runningState := func(pid uint32, finishedAt time.Time) *container.State {
		state := &container.State{FinishedAt: finishedAt}
		state.SetRunning(nil, &monitorMockTask{pid: pid}, currentStart)
		return state
	}

	tests := []struct {
		name       string
		state      *container.State
		event      libcontainerdtypes.EventInfo
		wantIgnore bool
	}{
		{
			name:  "different event PID ignores event when timestamp cannot identify it",
			state: runningState(200, previousExit),
			event: libcontainerdtypes.EventInfo{
				Pid: 100,
			},
			wantIgnore: true,
		},
		{
			name:  "same PID and previous exit timestamp ignores PID reuse duplicate",
			state: runningState(100, previousExit),
			event: libcontainerdtypes.EventInfo{
				Pid:      100,
				ExitedAt: previousExit,
			},
			wantIgnore: true,
		},
		{
			name:  "same PID with later exit timestamp processes current run",
			state: runningState(100, previousExit),
			event: libcontainerdtypes.EventInfo{
				Pid:      100,
				ExitedAt: currentExit,
			},
			wantIgnore: false,
		},
		{
			name:  "zero event PID processes conservatively",
			state: runningState(100, previousExit),
			event: libcontainerdtypes.EventInfo{
				Pid:      0,
				ExitedAt: currentExit,
			},
			wantIgnore: false,
		},
		{
			name:  "zero container PID processes conservatively",
			state: runningState(0, previousExit),
			event: libcontainerdtypes.EventInfo{
				Pid:      100,
				ExitedAt: currentExit,
			},
			wantIgnore: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			daemon := &Daemon{}
			ctr := &container.Container{
				ID:    "container_id",
				State: tc.state,
			}

			ctr.Lock()
			got := daemon.shouldIgnoreExitEventWithLock(ctr, &tc.event)
			ctr.Unlock()

			assert.Check(t, is.Equal(got, tc.wantIgnore))
		})
	}
}
