package equality

import (
	"crypto/subtle"

	"github.com/moby/swarmkit/v2/api"
)

// TasksEqualStable returns true if the tasks are functionally equal, ignoring status,
// version and other superfluous fields.
//
// This used to decide whether or not to propagate a task update to a controller.
func TasksEqualStable(a, b *api.Task) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	// Copy rather than clearing the fields on the originals and putting them
	// back: these objects are shared, so mutating them even briefly would race.
	// Enumerating the fields to compare instead would be cheaper but would
	// silently stop covering any field added to Task later.
	copyA, copyB := a.Copy(), b.Copy()

	copyA.Status, copyB.Status = nil, nil
	copyA.Meta, copyB.Meta = nil, nil

	return copyA.EqualVT(copyB)
}

// TaskStatusesEqualStable compares the task status excluding timestamp fields.
func TaskStatusesEqualStable(a, b *api.TaskStatus) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	copyA, copyB := a.Copy(), b.Copy()

	copyA.Timestamp, copyB.Timestamp = nil, nil
	return copyA.EqualVT(copyB)
}

// RootCAEqualStable compares RootCAs, excluding join tokens, which are randomly generated
func RootCAEqualStable(a, b *api.RootCA) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	var aRotationKey, bRotationKey []byte
	if a.RootRotation != nil {
		aRotationKey = a.RootRotation.CaKey
	}
	if b.RootRotation != nil {
		bRotationKey = b.RootRotation.CaKey
	}
	if subtle.ConstantTimeCompare(a.CaKey, b.CaKey) != 1 || subtle.ConstantTimeCompare(aRotationKey, bRotationKey) != 1 {
		return false
	}

	copyA, copyB := a.Copy(), b.Copy()
	copyA.JoinTokens, copyB.JoinTokens = nil, nil
	return copyA.EqualVT(copyB)
}

// ExternalCAsEqualStable compares lists of external CAs and determines whether they are equal.
func ExternalCAsEqualStable(a, b []*api.ExternalCA) bool {
	if len(a) != len(b) {
		return false
	}
	// Protobuf equality considers an unset map and an empty one equal, which is
	// what we want: both lists are assumed to have been deserialized from the
	// wire, where that distinction does not survive anyway.
	for i := range a {
		if !a[i].EqualVT(b[i]) {
			return false
		}
	}
	return true
}
