package runtime

import (
	"errors"
	"fmt"
)

// ErrRuntimeNotFound indicates that a runtime was not found locally.
type ErrRuntimeNotFound string

func (name ErrRuntimeNotFound) Error() string {
	return fmt.Sprintf("runtime %q not found", string(name))
}

// ErrRuntimeExists indicates that a runtime already exists.
type ErrRuntimeExists string

func (name ErrRuntimeExists) Error() string {
	return fmt.Sprintf("runtime %q already exists", string(name))
}

// ErrRuntimeDeleted indicates that a runtime was locally deleted.
type ErrRuntimeDeleted string

func (name ErrRuntimeDeleted) Error() string {
	return fmt.Sprintf("runtime %q was deleted", string(name))
}

var (
	ErrRuntimeMissingPlugin       = errors.New("missing plugin")
	ErrRuntimeMissingPluginGetter = errors.New("missing plugin getter")
)
