package container

import "time"

// StatusChanged returns a channel that is closed when the status changes or
// the container is removed.
// The caller must hold the state lock.
func (s *State) StatusChanged() <-chan struct{} {
	if s.statusChanged == nil {
		s.statusChanged = make(chan struct{})
	}
	return s.statusChanged
}

// StatusChangedAt returns when the status last changed or the container was
// removed, or the zero time if the daemon did not observe a change since it
// started.
// The caller must hold the state lock.
func (s *State) StatusChangedAt() time.Time {
	return s.statusChangedAt
}

// LastChange returns the most recent status change or the container's
// removal, and false if the daemon did not observe one since it started.
// The caller must hold the state lock.
func (s *State) LastChange() (StateChange, bool) {
	return s.lastChange, !s.statusChangedAt.IsZero()
}

// Deleted reports whether the container was removed.
// Unlike a failed removal, a removed container's state never changes again.
// The caller must hold the state lock.
func (s *State) Deleted() bool {
	return s.deleted
}
