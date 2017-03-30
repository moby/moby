package container

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
)

func TestIsValidHealthString(t *testing.T) {
	contexts := []struct {
		Health   string
		Expected bool
	}{
		{types.Healthy, true},
		{types.Unhealthy, true},
		{types.Starting, true},
		{types.NoHealthcheck, true},
		{"fail", false},
	}

	for _, c := range contexts {
		v := IsValidHealthString(c.Health)
		if v != c.Expected {
			t.Fatalf("Expected %t, but got %t", c.Expected, v)
		}
	}
}

func TestStateRunStop(t *testing.T) {
	s := NewState()

	// An initial wait (in "created" state) should block until the
	// container has started and exited. It shouldn't take more than 100
	// milliseconds.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	initialWait := s.Wait(ctx, false)

	// Begin another wait for the final removed state. It should complete
	// within 200 milliseconds.
	ctx, cancel = context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	removalWait := s.Wait(ctx, true)

	// Full lifecycle two times.
	for i := 1; i <= 2; i++ {
		// Set the state to "Running".
		s.Lock()
		s.SetRunning(i, true)
		s.Unlock()

		// Assert desired state.
		if !s.IsRunning() {
			t.Fatal("State not running")
		}
		if s.Pid != i {
			t.Fatalf("Pid %v, expected %v", s.Pid, i)
		}
		if s.ExitCode() != 0 {
			t.Fatalf("ExitCode %v, expected 0", s.ExitCode())
		}

		// Async wait up to 50 milliseconds for the exit status.
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		exitWait := s.Wait(ctx, false)

		// Set the state to "Exited".
		s.Lock()
		s.SetStopped(&ExitStatus{ExitCode: i})
		s.Unlock()

		// Assert desired state.
		if s.IsRunning() {
			t.Fatal("State is running")
		}
		if s.ExitCode() != i {
			t.Fatalf("ExitCode %v, expected %v", s.ExitCode(), i)
		}
		if s.Pid != 0 {
			t.Fatalf("Pid %v, expected 0", s.Pid)
		}

		// Receive the exitWait result.
		status := <-exitWait
		if status.ExitCode() != i {
			t.Fatalf("ExitCode %v, expected %v, err %q", status.ExitCode(), i, status.Err())
		}

		// A repeated call to Wait() should not block at this point.
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		exitWait = s.Wait(ctx, false)

		status = <-exitWait
		if status.ExitCode() != i {
			t.Fatalf("ExitCode %v, expected %v, err %q", status.ExitCode(), i, status.Err())
		}

		if i == 1 {
			// Make sure our initial wait also succeeds.
			status = <-initialWait
			if status.ExitCode() != i {
				// Should have the exit code from this first loop.
				t.Fatalf("Initial wait exitCode %v, expected %v, err %q", status.ExitCode(), i, status.Err())
			}
		}
	}

	// Set the state to dead and removed.
	s.SetDead()
	s.SetRemoved()

	// Wait for removed status or timeout.
	status := <-removalWait
	if status.ExitCode() != 2 {
		// Should have the final exit code from the loop.
		t.Fatalf("Removal wait exitCode %v, expected %v, err %q", status.ExitCode(), 2, status.Err())
	}
}

func TestStateTimeoutWait(t *testing.T) {
	s := NewState()

	// Start a wait with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	waitC := s.Wait(ctx, false)

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
	s.SetRunning(0, true)
	s.SetStopped(&ExitStatus{ExitCode: 0})
	s.Unlock()

	// Start another wait with a timeout. This one should return
	// immediately.
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	waitC = s.Wait(ctx, false)

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
