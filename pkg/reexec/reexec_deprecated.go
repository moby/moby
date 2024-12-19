// Package reexec facilitates the busybox style reexec of a binary.
//
// Deprecated: this package is deprecated and moved to a separate module. Use [github.com/moby/sys/reexec] instead.
package reexec

import (
	"os/exec"

	"github.com/moby/sys/reexec"
)

// Register adds an initialization func under the specified name. It panics
// if the given name is already registered.
//
// Deprecated: use [reexec.Register] instead.
func Register(name string, initializer func()) {
	reexec.Register(name, initializer)
}

// Init is called as the first part of the exec process and returns true if an
// initialization function was called.
//
// Deprecated: use [reexec.Init] instead.
func Init() bool {
	return reexec.Init()
}

// Command returns an [*exec.Cmd] with its Path set to the path of the current
// binary using the result of [Self].
//
// Deprecated: use [reexec.Command] instead.
func Command(args ...string) *exec.Cmd {
	return reexec.Command(args...)
}

// Self returns the path to the current process's binary.
//
// Deprecated: use [reexec.Self] instead.
func Self() string {
	return reexec.Self()
}
