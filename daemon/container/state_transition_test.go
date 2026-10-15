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

	health := &Health{}
	health.SetStatus(containertypes.Starting)
	s := &State{Health: health}
	transition := func(tr Transition) (StateChange, error) {
		s.Lock()
		defer s.Unlock()

		return s.Transition(tr)
	}
	current := func() StatusResult {
		t.Helper()

		for r, err := range s.StatusChanges(t.Context()) {
			assert.NilError(t, err)
			return r
		}

		t.Fatal("no state was yielded")

		return StatusResult{}
	}

	r := current()
	assert.Assert(t, r.ChangedAt.IsZero())
	assert.Equal(t, r.Current.Status, containertypes.StateCreated)

	_, err := transition(Transition{To: containertypes.StatePaused})
	assert.Assert(t, cerrdefs.IsConflict(err))

	change, err := transition(Transition{To: containertypes.StateRunning, StartedAt: time.Now()})
	assert.NilError(t, err)
	assert.Equal(t, change.Previous.Status, containertypes.StateCreated)
	assert.Equal(t, change.Current.Status, containertypes.StateRunning)

	_, err = transition(Transition{To: containertypes.StateRunning, StartedAt: time.Now()})
	assert.Assert(t, cerrdefs.IsConflict(err))

	_, err = transition(Transition{To: containertypes.StateExited, ExitCode: 3, Error: errors.New("exit error")})
	assert.NilError(t, err)

	exitedAt := current().ChangedAt
	// A transition that keeps the status is not a status change.
	_, err = transition(Transition{To: containertypes.StateExited, ExitCode: 3})
	assert.NilError(t, err)
	assert.Equal(t, current().ChangedAt, exitedAt)

	// The previous state keeps the exit that starting resets, and has the
	// health as of the change.
	health.SetStatus(containertypes.Unhealthy)
	assert.Equal(t, current().Current.Health, containertypes.Unhealthy)

	_, err = transition(Transition{To: containertypes.StateRunning, StartedAt: time.Now()})
	assert.NilError(t, err)

	r = current()
	assert.Equal(t, r.Previous.Status, containertypes.StateExited)
	assert.Equal(t, r.Previous.ExitCode, 3)
	assert.Equal(t, r.Previous.Error, "exit error")
	assert.Equal(t, r.Previous.Health, containertypes.Unhealthy)
	assert.Equal(t, r.Current.ExitCode, 0)
	assert.Equal(t, r.Current.Error, "")

	_, err = transition(Transition{To: containertypes.StateExited, ExitCode: 7, Error: errors.New("exit error")})
	assert.NilError(t, err)
	_, err = transition(Transition{To: containertypes.StateRemoving})
	assert.NilError(t, err)
	_, err = transition(Transition{To: containertypes.StateRemoving})
	assert.Assert(t, cerrdefs.IsConflict(err))
	assert.Assert(t, !current().Removed)

	s.SetRemoved()

	removed := current()
	assert.Assert(t, removed.Removed)

	// containerRm resets RemovalInProgress after the container is removed.
	s.ResetRemovalInProgress()

	r = current()
	assert.Equal(t, r.ChangedAt, removed.ChangedAt)
	assert.Equal(t, r.Current.Status, containertypes.StateRemoving)
	// Removal clears the error, but the previous state keeps it.
	assert.Equal(t, r.Previous.Status, containertypes.StateRemoving)
	assert.Equal(t, r.Previous.Error, "exit error")
	assert.Equal(t, r.Current.Error, "")
}
