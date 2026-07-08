package container

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type mockTask struct {
	libcontainerdtypes.Task
	pid uint32
}

func (t *mockTask) Pid() uint32 { return t.pid }

func TestStateRunStop(t *testing.T) {
	s := &State{}

	// Begin another wait with WaitConditionRemoved. It should complete
	// within 200 milliseconds.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	removalWait := s.Wait(ctx, container.WaitConditionRemoved)

	// Full lifecycle two times.
	for i := 1; i <= 2; i++ {
		// A wait with WaitConditionNotRunning should return
		// immediately since the state is now either "created" (on the
		// first iteration) or "exited" (on the second iteration). It
		// shouldn't take more than 50 milliseconds.
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		// Expectx exit code to be i-1 since it should be the exit
		// code from the previous loop or 0 for the created state.
		if status := <-s.Wait(ctx, container.WaitConditionNotRunning); status.ExitCode() != i-1 {
			t.Fatalf("ExitCode %v, expected %v, err %q", status.ExitCode(), i-1, status.Err())
		}

		// A wait with WaitConditionNextExit should block until the
		// container has started and exited. It shouldn't take more
		// than 100 milliseconds.
		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		initialWait := s.Wait(ctx, container.WaitConditionNextExit)

		// Set the state to "Running".
		s.Lock()
		s.SetRunning(nil, &mockTask{pid: uint32(i)}, time.Now())
		s.Unlock()

		// Assert desired state.
		if !s.IsRunning() {
			t.Fatal("State not running")
		}
		if s.Pid != i {
			t.Fatalf("Pid %v, expected %v", s.Pid, i)
		}
		if s.ExitCode != 0 {
			t.Fatalf("ExitCode %v, expected 0", s.ExitCode)
		}

		// Now that it's running, a wait with WaitConditionNotRunning
		// should block until we stop the container. It shouldn't take
		// more than 100 milliseconds.
		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		exitWait := s.Wait(ctx, container.WaitConditionNotRunning)

		// Set the state to "Exited".
		s.Lock()
		s.SetStopped(&ExitStatus{ExitCode: i})
		s.Unlock()

		// Assert desired state.
		if s.IsRunning() {
			t.Fatal("State is running")
		}
		if s.ExitCode != i {
			t.Fatalf("ExitCode %v, expected %v", s.ExitCode, i)
		}
		if s.Pid != 0 {
			t.Fatalf("Pid %v, expected 0", s.Pid)
		}

		// Receive the initialWait result.
		if status := <-initialWait; status.ExitCode() != i {
			t.Fatalf("ExitCode %v, expected %v, err %q", status.ExitCode(), i, status.Err())
		}

		// Receive the exitWait result.
		if status := <-exitWait; status.ExitCode() != i {
			t.Fatalf("ExitCode %v, expected %v, err %q", status.ExitCode(), i, status.Err())
		}
	}

	// Set the state to dead and removed.
	s.Lock()
	s.Dead = true
	s.Unlock()
	s.SetRemoved()

	// Wait for removed status or timeout.
	if status := <-removalWait; status.ExitCode() != 2 {
		// Should have the final exit code from the loop.
		t.Fatalf("Removal wait exitCode %v, expected %v, err %q", status.ExitCode(), 2, status.Err())
	}
}

func TestStateTimeoutWait(t *testing.T) {
	s := &State{}

	s.Lock()
	s.SetRunning(nil, nil, time.Now())
	s.Unlock()

	// Start a wait with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	waitC := s.Wait(ctx, container.WaitConditionNotRunning)

	// It should timeout *before* this 200ms timer does.
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop callback doesn't fire in 200 milliseconds")
	case status := <-waitC:
		t.Log("Stop callback fired")
		// Should be a timeout error.
		if status.Err() == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if status.ExitCode() != -1 {
			t.Fatalf("expected exit code %v, got %v", -1, status.ExitCode())
		}
	}

	s.Lock()
	s.SetStopped(&ExitStatus{ExitCode: 0})
	s.Unlock()

	// Start another wait with a timeout. This one should return
	// immediately.
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	waitC = s.Wait(ctx, container.WaitConditionNotRunning)

	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop callback doesn't fire in 200 milliseconds")
	case status := <-waitC:
		t.Log("Stop callback fired")
		if status.ExitCode() != 0 {
			t.Fatalf("expected exit code %v, got %v, err %q", 0, status.ExitCode(), status.Err())
		}
	}
}

// TestStateWaitReturnsExitStatusAfterRestart verifies that Wait returns the
// correct exit status when a container is restarted immediately after exiting,
// covering a race where Wait could observe the restarted state instead.
//
// Related issue: https://github.com/moby/moby/issues/39352
func TestStateWaitReturnsExitStatusAfterRestart(t *testing.T) {
	s := &State{}

	s.Lock()
	s.SetRunning(nil, nil, time.Now())
	s.Unlock()

	waitC := s.Wait(context.Background(), container.WaitConditionNotRunning)
	want := ExitStatus{ExitCode: 10, ExitedAt: time.Now()}

	s.Lock()
	s.SetRestarting(&want)
	s.Unlock()

	s.Lock()
	s.SetRunning(nil, nil, time.Now())
	s.Unlock()

	got := <-waitC
	if got.ExitCode() != want.ExitCode {
		t.Fatalf("expected exit code %v, got %v", want.ExitCode, got.ExitCode())
	}
}

func TestStateStateAndString(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		doc        string
		state      *State
		wantState  container.ContainerState
		wantString string
	}{
		{
			doc:        "removal in progress",
			state:      &State{RemovalInProgress: true},
			wantState:  container.StateRemoving,
			wantString: "Removal In Progress",
		},
		{
			doc:        "dead",
			state:      &State{Dead: true},
			wantState:  container.StateDead,
			wantString: "Dead",
		},
		{
			doc:        "restarting",
			state:      &State{Running: true, Restarting: true, ExitCode: 1, FinishedAt: now.Add(-72 * time.Hour)},
			wantState:  container.StateRestarting,
			wantString: "Restarting (1) 3 days ago",
		},
		{
			doc:        "running",
			state:      &State{Running: true, StartedAt: now.Add(-72 * time.Hour)},
			wantState:  container.StateRunning,
			wantString: "Up 3 days",
		},
		{
			doc:        "paused",
			state:      &State{Running: true, Paused: true, StartedAt: now.Add(-72 * time.Hour)},
			wantState:  container.StatePaused,
			wantString: "Up 3 days (Paused)",
		},
		{
			doc:        "created",
			state:      &State{},
			wantState:  container.StateCreated,
			wantString: "Created",
		},
		{
			doc:        "exited",
			state:      &State{StartedAt: now.Add(-96 * time.Hour), FinishedAt: now.Add(-72 * time.Hour), ExitCode: 137},
			wantState:  container.StateExited,
			wantString: "Exited (137) 3 days ago",
		},
		{
			doc:        "started but not finished",
			state:      &State{StartedAt: now.Add(-72 * time.Hour)},
			wantState:  container.StateExited,
			wantString: "",
		},
		{
			doc:        "restarting while not running",
			state:      &State{Restarting: true, StartedAt: now.Add(-96 * time.Hour), FinishedAt: now.Add(-72 * time.Hour), ExitCode: 1},
			wantState:  container.StateExited,
			wantString: "Exited (1) 3 days ago",
		},
		{
			doc:        "paused while not running",
			state:      &State{Paused: true},
			wantState:  container.StateCreated,
			wantString: "Created",
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			assert.Check(t, is.Equal(tc.state.State(), tc.wantState))
			assert.Check(t, is.Equal(tc.state.String(), tc.wantString))
		})
	}
}
