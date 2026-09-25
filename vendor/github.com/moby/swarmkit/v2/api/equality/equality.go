package equality

import (
	"crypto/subtle"
	"reflect"

	"github.com/moby/swarmkit/v2/api"
)

// TasksEqualStable returns true if the tasks are functionally equal, ignoring status,
// version and other superfluous fields.
//
// This used to decide whether or not to propagate a task update to a controller.
func TasksEqualStable(a, b *api.Task) bool {
	// shallow copy
	copyA, copyB := *a, *b

	copyA.Status, copyB.Status = api.TaskStatus{}, api.TaskStatus{}
	copyA.Meta, copyB.Meta = api.Meta{}, api.Meta{}

	return reflect.DeepEqual(&copyA, &copyB)
}

// TaskStatusesEqualStable compares the task status excluding timestamp fields.
func TaskStatusesEqualStable(a, b *api.TaskStatus) bool {
	copyA, copyB := *a, *b

	copyA.Timestamp, copyB.Timestamp = nil, nil
	return reflect.DeepEqual(&copyA, &copyB)
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
		aRotationKey = a.RootRotation.CAKey
	}
	if b.RootRotation != nil {
		bRotationKey = b.RootRotation.CAKey
	}
	if subtle.ConstantTimeCompare(a.CAKey, b.CAKey) != 1 || subtle.ConstantTimeCompare(aRotationKey, bRotationKey) != 1 {
		return false
	}

	copyA, copyB := *a, *b
	copyA.JoinTokens, copyB.JoinTokens = api.JoinTokens{}, api.JoinTokens{}
	return reflect.DeepEqual(copyA, copyB)
}

// ExternalCAsEqualStable compares lists of external CAs and determines whether they are equal.
func ExternalCAsEqualStable(a, b []*api.ExternalCA) bool {
	// because DeepEqual will treat an empty list and a nil list differently, we want to manually check this first
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	// The assumption is that each individual api.ExternalCA within both lists are created from deserializing from a
	// protobuf, so no special affordances are made to treat a nil map and empty map in the Options field of an
	// api.ExternalCA as equivalent.
	return reflect.DeepEqual(a, b)
}
