package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/docker/go-events"
)

var (
	errUnknownStoreAction = errors.New("unrecognized action type")
	errConflictingFilters = errors.New("conflicting filters specified")
	errNoKindSpecified    = errors.New("no kind of object specified")
	errUnrecognizedAction = errors.New("unrecognized action")
)

// StoreObject is an abstract object that can be handled by the store.
type StoreObject interface {
	GetID() string                           // Get ID
	GetMeta() Meta                           // Retrieve metadata
	SetMeta(Meta)                            // Set metadata
	CopyStoreObject() StoreObject            // Return a copy of this object
	EventCreate() Event                      // Return a creation event
	EventUpdate(oldObject StoreObject) Event // Return an update event
	EventDelete() Event                      // Return a deletion event
}

// Event is the type used for events passed over watcher channels, and also
// the type used to specify filtering in calls to Watch.
type Event interface {
	// TODO(stevvooe): Consider whether it makes sense to squish both the
	// matcher type and the primary type into the same type. It might be better
	// to build a matcher from an event prototype.

	// Matches checks if this item in a watch queue Matches the event
	// description.
	Matches(events.Event) bool
}

func customIndexer(kind string, annotations *Annotations) (bool, [][]byte, error) {
	var converted [][]byte

	for _, entry := range annotations.Indices {
		index := make([]byte, 0, len(kind)+1+len(entry.Key)+1+len(entry.Val)+1)
		if kind != "" {
			index = append(index, []byte(kind)...)
			index = append(index, '|')
		}
		index = append(index, []byte(entry.Key)...)
		index = append(index, '|')
		index = append(index, []byte(entry.Val)...)
		index = append(index, '\x00')
		converted = append(converted, index)
	}

	// Add the null character as a terminator
	return len(converted) != 0, converted, nil
}

func fromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func prefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := fromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

func checkCustom(a1, a2 Annotations) bool {
	if len(a1.Indices) == 1 {
		for _, ind := range a2.Indices {
			if ind.Key == a1.Indices[0].Key && ind.Val == a1.Indices[0].Val {
				return true
			}
		}
	}
	return false
}

func checkCustomPrefix(a1, a2 Annotations) bool {
	if len(a1.Indices) == 1 {
		for _, ind := range a2.Indices {
			if ind.Key == a1.Indices[0].Key && strings.HasPrefix(ind.Val, a1.Indices[0].Val) {
				return true
			}
		}
	}
	return false
}
