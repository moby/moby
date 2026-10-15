package container

import (
	"context"
	"iter"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
)

func TestStateStatusChanges(t *testing.T) {
	t.Parallel()

	s := &State{}
	transition := func(transitions ...Transition) {
		t.Helper()
		s.Lock()
		defer s.Unlock()

		for _, tr := range transitions {
			_, err := s.Transition(tr)
			assert.NilError(t, err)
		}
	}
	subscribers := func() int {
		s.Lock()
		defer s.Unlock()

		return len(s.statusSubscribers)
	}
	must := func(r StatusResult, err error, ok bool) StatusResult {
		t.Helper()
		assert.Assert(t, ok)
		assert.NilError(t, err)

		return r
	}

	transition(Transition{To: containertypes.StateRunning, StartedAt: time.Now()})

	// Every status change is yielded in order, even if several happen before
	// the consumer reads them.
	next, stop := iter.Pull2(s.StatusChanges(t.Context()))
	assert.Equal(t, must(next()).Current.Status, containertypes.StateRunning)
	transition(
		Transition{To: containertypes.StateExited, ExitCode: 3},
		Transition{To: containertypes.StateRunning, StartedAt: time.Now()},
		Transition{To: containertypes.StatePaused},
	)

	r := must(next())
	assert.Equal(t, r.Current.Status, containertypes.StateExited)
	assert.Equal(t, r.Current.ExitCode, 3)
	assert.Equal(t, r.Previous.Status, containertypes.StateRunning)
	assert.Equal(t, must(next()).Current.Status, containertypes.StateRunning)
	assert.Equal(t, must(next()).Current.Status, containertypes.StatePaused)
	stop()
	assert.Equal(t, subscribers(), 0)

	// A consumer that falls too far behind gets an error instead of skipped
	// changes, and is unsubscribed.
	next, stop = iter.Pull2(s.StatusChanges(t.Context()))
	defer stop()

	must(next())

	for i := range maxPendingStatusChanges + 1 {
		if i%2 == 0 {
			transition(Transition{To: containertypes.StateRunning})
		} else {
			transition(Transition{To: containertypes.StatePaused})
		}
	}

	for range maxPendingStatusChanges {
		must(next())
	}

	_, err, _ := next()
	assert.Assert(t, cerrdefs.IsUnavailable(err))

	_, _, ok := next()
	assert.Assert(t, !ok)
	assert.Equal(t, subscribers(), 0)

	// Removal is the last change.
	next, stop = iter.Pull2(s.StatusChanges(t.Context()))
	defer stop()

	must(next())
	s.SetRemoved()
	assert.Assert(t, must(next()).Removed)

	_, _, ok = next()
	assert.Assert(t, !ok)
}

func TestStateStatusChangesCancellationWithPendingChanges(t *testing.T) {
	t.Parallel()

	s := &State{Running: true}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	next, stop := iter.Pull2(s.StatusChanges(ctx))
	defer stop()

	_, err, ok := next()
	assert.NilError(t, err)
	assert.Assert(t, ok)

	s.Lock()
	_, err = s.Transition(Transition{To: containertypes.StatePaused})
	s.Unlock()
	assert.NilError(t, err)

	cancel()

	_, err, ok = next()
	assert.ErrorIs(t, err, context.Canceled)
	assert.Assert(t, ok)

	_, _, ok = next()
	assert.Assert(t, !ok)

	s.Lock()
	defer s.Unlock()

	assert.Equal(t, len(s.statusSubscribers), 0)
}
