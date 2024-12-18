// Package reexec facilitates the busybox style reexec of a binary.
//
// Handlers can be registered with a name and the argv 0 of the exec of
// the binary will be used to find and execute custom init paths.
//
// It is used to work around forking limitations when using Go.
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

// Command returns an [*exec.Cmd] with its Path set to the path of the current
// binary using the result of [Self].
//
// On Linux, the Pdeathsig of [*exec.Cmd.SysProcAttr] is set to SIGTERM.
// This signal is sent to the process when the OS thread that created
// the process dies.
//
// It is the caller's responsibility to ensure that the creating thread is
// not terminated prematurely. See https://go.dev/issue/27505 for more details.
func Command(args ...string) *exec.Cmd {
	return command(args...)
}

// Self returns the path to the current process's binary.
//
// On Linux, it returns "/proc/self/exe", which provides the in-memory version
// of the current binary. This makes it safe to delete or replace the on-disk
// binary (os.Args[0]).
//
// On Other platforms, it attempts to look up the absolute path for os.Args[0],
// or otherwise returns os.Args[0] as-is. For example if current binary is
// "my-binary" at "/usr/bin/" (or "my-binary.exe" at "C:\" on Windows),
// then it returns "/usr/bin/my-binary" and "C:\my-binary.exe" respectively.
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
