// Package reexec facilitates the busybox style reexec of a binary.
//
// Handlers can be registered with a name and the argv 0 of the exec of
// the binary will be used to find and execute custom init paths.
//
// It is used in dockerd to work around forking limitations when using Go.
package reexec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var registeredInitializers = make(map[string]func())

// Register adds an initialization func under the specified name. It panics
// if the given name is already registered.
func Register(name string, initializer func()) {
	if _, exists := registeredInitializers[name]; exists {
		panic(fmt.Sprintf("reexec func already registered under name %q", name))
	}

	registeredInitializers[name] = initializer
}

// Init is called as the first part of the exec process and returns true if an
// initialization function was called.
func Init() bool {
	if initializer, ok := registeredInitializers[os.Args[0]]; ok {
		initializer()
		return true
	}
	return false
}

// Self returns the path to the current process's binary. On Linux, it
// returns "/proc/self/exe", which provides the in-memory version of the
// current binary, whereas on other platforms it attempts to looks up the
// absolute path for os.Args[0], or otherwise returns os.Args[0] as-is.
func Self() string {
	if runtime.GOOS == "linux" {
		return "/proc/self/exe"
	}
	return naiveSelf()
}

func naiveSelf() string {
	name := os.Args[0]
	if filepath.Base(name) == name {
		if lp, err := exec.LookPath(name); err == nil {
			return lp
		}
	}
	// handle conversion of relative paths to absolute
	if absName, err := filepath.Abs(name); err == nil {
		return absName
	}
	// if we couldn't get absolute name, return original
	// (NOTE: Go only errors on Abs() if os.Getwd fails)
	return name
}
