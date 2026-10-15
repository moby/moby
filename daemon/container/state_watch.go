package container

import (
	"context"
	"errors"
	"iter"
	"slices"
	"time"

	"github.com/moby/moby/v2/errdefs"
)

// maxPendingStatusChanges is how many status changes a consumer of
// [State.StatusChanges] can fall behind before the iteration ends.
const maxPendingStatusChanges = 64

// StatusResult is a container's state yielded by [State.StatusChanges].
type StatusResult struct {
	// Current is the container's state after the status change, or when the
	// iteration started.
	Current StateSnapshot
	// Previous is the state just before the status last changed, and
	// ChangedAt is when it changed.
	// Both are zero if the daemon did not observe a change since it started.
	Previous  StateSnapshot
	ChangedAt time.Time
	// Removed reports whether the container was removed.
	// Unlike after a failed removal, a removed container's state never
	// changes again.
	Removed bool
}

type statusSubscriber struct {
	pending []StatusResult
	ready   chan struct{}
	err     error
}

// StatusChanges yields the container's state, then its state after each
// status change, in order, and stops after the container is removed.
// It yields an error and stops if ctx is done, or if the consumer falls more
// than [maxPendingStatusChanges] changes behind, instead of skipping changes.
// Breaking out of the loop unsubscribes.
// It must be called without holding the state lock, and yields without
// holding it.
func (s *State) StatusChanges(ctx context.Context) iter.Seq2[StatusResult, error] {
	return func(yield func(StatusResult, error) bool) {
		sub := &statusSubscriber{ready: make(chan struct{}, 1)}

		s.Lock()

		r := s.currentStatus()
		if !r.Removed {
			s.statusSubscribers = append(s.statusSubscribers, sub)
		}
		s.Unlock()

		defer s.unsubscribe(sub)

		for {
			if !yield(r, nil) || r.Removed {
				return
			}

			var err error
			if r, err = s.nextStatus(ctx, sub); err != nil {
				yield(StatusResult{}, err)
				return
			}
		}
	}
}

// currentStatus returns the container's state and its last status change.
// The caller must hold the state lock.
func (s *State) currentStatus() StatusResult {
	r := StatusResult{
		Current:   s.snapshot(),
		Previous:  s.lastChange.Previous,
		ChangedAt: s.statusChangedAt,
		Removed:   s.deleted,
	}
	if r.Removed {
		// Changes after removal, such as resetting RemovalInProgress, are
		// not recorded.
		r.Current = s.lastChange.Current
	}

	return r
}

// nextStatus waits for the next status change queued for sub.
func (s *State) nextStatus(ctx context.Context, sub *statusSubscriber) (StatusResult, error) {
	for {
		if err := ctx.Err(); err != nil {
			return StatusResult{}, err
		}

		s.Lock()
		if len(sub.pending) > 0 {
			r := sub.pending[0]
			sub.pending = slices.Delete(sub.pending, 0, 1)
			s.Unlock()

			return r, nil
		}

		err := sub.err
		s.Unlock()

		if err != nil {
			return StatusResult{}, err
		}

		select {
		case <-ctx.Done():
			return StatusResult{}, ctx.Err()
		case <-sub.ready:
		}
	}
}

func (s *State) unsubscribe(sub *statusSubscriber) {
	s.Lock()
	defer s.Unlock()

	s.statusSubscribers = slices.DeleteFunc(s.statusSubscribers, func(o *statusSubscriber) bool { return o == sub })
}

// notifyStatusSubscribers queues the recorded change for every subscriber.
// A subscriber that fell too far behind is dropped with an error instead.
// The caller must hold the state lock.
func (s *State) notifyStatusSubscribers() {
	r := StatusResult{
		Current:   s.lastChange.Current,
		Previous:  s.lastChange.Previous,
		ChangedAt: s.statusChangedAt,
		Removed:   s.deleted,
	}
	s.statusSubscribers = slices.DeleteFunc(s.statusSubscribers, func(sub *statusSubscriber) bool {
		behind := len(sub.pending) == maxPendingStatusChanges
		if behind {
			sub.err = errdefs.Unavailable(errors.New("container status changes were not consumed fast enough"))
		} else {
			sub.pending = append(sub.pending, r)
		}

		select {
		case sub.ready <- struct{}{}:
		default:
		}

		return behind
	})
}
