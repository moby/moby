package container

import (
	"errors"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
)

func TestStateTransition(t *testing.T) {
	t.Parallel()

	assertClosed := func(changed <-chan struct{}) {
		t.Helper()
		select {
		case <-changed:
		default:
			t.Fatal("status change did not close the channel")
		}
	}

	health := &Health{}
	health.SetStatus(containertypes.Starting)
	s := &State{Health: health}
	s.Lock()
	_, ok := s.LastChange()
	assert.Assert(t, !ok)
	_, err := s.Transition(Transition{To: containertypes.StatePaused})
	assert.Assert(t, cerrdefs.IsConflict(err))

	change, err := s.Transition(Transition{To: containertypes.StateRunning, StartedAt: time.Now()})
	assert.NilError(t, err)
	assert.Equal(t, change.Previous.Status, containertypes.StateCreated)
	assert.Equal(t, change.Current.Status, containertypes.StateRunning)
	runningAt := s.StatusChangedAt()
	// A transition that keeps the status is not a status change.
	_, err = s.Transition(Transition{To: containertypes.StateRunning, StartedAt: time.Now()})
	assert.NilError(t, err)
	assert.Equal(t, s.StatusChangedAt(), runningAt)

	changed := s.StatusChanged()
	_, err = s.Transition(Transition{To: containertypes.StateExited, ExitCode: 3, Error: errors.New("exit error")})
	assert.NilError(t, err)
	assertClosed(changed)

	// The previous state keeps the exit that starting resets, and has the
	// health as of the change.
	health.SetStatus(containertypes.Unhealthy)
	_, err = s.Transition(Transition{To: containertypes.StateRunning, StartedAt: time.Now()})
	assert.NilError(t, err)
	change, ok = s.LastChange()
	assert.Assert(t, ok)
	assert.Equal(t, change.Previous.Status, containertypes.StateExited)
	assert.Equal(t, change.Previous.ExitCode, 3)
	assert.Equal(t, change.Previous.Error, "exit error")
	assert.Equal(t, change.Previous.Health, containertypes.Unhealthy)
	assert.Equal(t, change.Current.ExitCode, 0)
	assert.Equal(t, change.Current.Error, "")

	_, err = s.Transition(Transition{To: containertypes.StateExited, ExitCode: 7, Error: errors.New("exit error")})
	assert.NilError(t, err)
	_, err = s.Transition(Transition{To: containertypes.StateRemoving})
	assert.NilError(t, err)
	_, err = s.Transition(Transition{To: containertypes.StateRemoving})
	assert.Assert(t, cerrdefs.IsConflict(err))
	changed = s.StatusChanged()
	assert.Assert(t, !s.Deleted())
	s.Unlock()

	s.SetRemoved()
	assertClosed(changed)
	s.Lock()
	assert.Assert(t, s.Deleted())
	removedAt := s.StatusChangedAt()
	s.Unlock()

	// containerRm resets RemovalInProgress after the container is removed.
	s.ResetRemovalInProgress()
	s.Lock()
	defer s.Unlock()
	assert.Equal(t, s.StatusChangedAt(), removedAt)
	// Removal clears the error, but the previous state keeps it.
	change, _ = s.LastChange()
	assert.Equal(t, change.Previous.Status, containertypes.StateRemoving)
	assert.Equal(t, change.Previous.Error, "exit error")
	assert.Equal(t, change.Current.Error, "")
}
