// Package reexec implements a BusyBox-style “multi-call binary” re-execution
// pattern for Go programs.
//
// Callers register named entrypoints through [Register]. When the current
// process starts, [Init] compares filepath.Base(os.Args[0]) against the set of
// registered entrypoints. If it matches, Init runs that entrypoint and returns
// true.
//
// A matched entrypoint is analogous to main for that invocation mode: callers
// typically call Init at the start of main and return immediately when it
// reports a match.
//
// Example uses:
//
//   - For multi-call binaries: multiple symlinks/hardlinks point at one binary,
//     and argv[0] (see [execve(2)]) selects the entrypoint.
//   - For programmatic re-exec: a parent launches the current binary (via
//     [Command] or [CommandContext]) with argv[0] set to a registered entrypoint
//     name to run a specific mode.
//
// The programmatic re-exec pattern is commonly used as a safe alternative to
// fork-without-exec in Go, since the runtime is not fork-safe once multiple
// threads exist (see [os.StartProcess] and https://go.dev/issue/27505).
//
// Example (multi-call binary):
//
//	package main
//
//	import (
//		"fmt"
//
//		"github.com/moby/sys/reexec"
//	)
//
//	func init() {
//		reexec.Register("example-foo", func() {
//			fmt.Println("Hello from entrypoint example-foo")
//		})
//		reexec.Register("example-bar", entrypointBar)
//	}
//
//	func main() {
//		if reexec.Init() {
//			// Matched a reexec entrypoint; stop normal main execution.
//			return
//		}
//		fmt.Println("Hello main")
//	}
//
//	func entrypointBar() {
//		fmt.Println("Hello from entrypoint example-bar")
//	}
//
// To try it:
//
//	go build -o example .
//	ln -s example example-foo
//	ln -s example example-bar
//
//	./example
//	# Hello main
//
//	./example-foo
//	# Hello from entrypoint example-foo
//
//	./example-bar
//	# Hello from entrypoint example-bar
//
// [execve(2)]: https://man7.org/linux/man-pages/man2/execve.2.html
package reexec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/moby/sys/reexec/internal/reexecoverride"
)

var entrypoints = make(map[string]func(context.Context) error)

// Register associates name with an entrypoint function to be executed when
// the current binary is invoked with argv[0] equal to name.
//
// Register is not safe for concurrent use; entrypoints must be registered
// during program initialization, before calling [Dispatch] or [Init].
// It panics if name contains a path component or is already registered.
func Register(name string, entrypoint func()) {
	registerEntrypoint(name, func(context.Context) error {
		entrypoint()
		return nil
	})
}

// RegisterContext is like [Register] but the entrypoint receives a context and
// reports success or failure by returning an error.
//
// RegisterContext is not safe for concurrent use; entrypoints must be
// registered during program initialization, before calling [Dispatch] or [Init].
// It panics if name contains a path component or is already registered.
func RegisterContext(name string, entrypoint func(context.Context) error) {
	registerEntrypoint(name, entrypoint)
}

func registerEntrypoint(name string, entrypoint func(context.Context) error) {
	if filepath.Base(name) != name {
		panic(fmt.Sprintf("reexec func does not expect a path component: %q", name))
	}
	if _, exists := entrypoints[name]; exists {
		panic(fmt.Sprintf("reexec func already registered under name %q", name))
	}

	entrypoints[name] = entrypoint
}

// Dispatch checks whether the current process was invoked under a registered
// name (based on filepath.Base(os.Args[0])).
//
// If a matching entrypoint is found, it is executed and Dispatch reports a
// match with ok. If ok is false, err is always nil. If ok is true, err is the
// entrypoint's return value.
//
// In the ok=true case, the caller should stop normal main execution (typically
// by returning from main or calling os.Exit).
func Dispatch(ctx context.Context) (bool, error) {
	return dispatch(ctx)
}

// Init checks whether the current process was invoked under a registered name
// (based on filepath.Base(os.Args[0])).
//
// If a matching entrypoint is found, it is executed and Init returns true. In
// that case, the caller should stop normal main execution. If no match is found,
// Init returns false and normal execution should continue.
//
// Init is provided for compatibility with older callers. New code should
// prefer [Dispatch], which also return the entrypoint's error and allows
// the caller to decide how to report failures.
func Init() bool {
	ok, _ := dispatch(context.Background())
	return ok
}

func dispatch(ctx context.Context) (bool, error) {
	entrypoint, ok := entrypoints[filepath.Base(os.Args[0])]
	if !ok {
		return false, nil
	}
	return true, entrypoint(ctx)
}

// Command returns an [*exec.Cmd] configured to re-execute the current binary,
// using the path returned by [Self].
//
// The first element of args becomes argv[0] of the new process and is used by
// [Init] to select a registered entrypoint.
//
// On Linux, the Pdeathsig of [*exec.Cmd.SysProcAttr] is set to SIGTERM. This
// signal is sent to the child process when the OS thread that created it dies,
// which helps ensure the child does not outlive its parent unexpectedly. See
// [PR_SET_PDEATHSIG(2const)] and [go.dev/issue/27505] for details.
//
// It is the caller's responsibility to ensure that the creating thread is not
// terminated prematurely.
//
// [PR_SET_PDEATHSIG(2const)]: https://man7.org/linux/man-pages/man2/PR_SET_PDEATHSIG.2const.html
// [go.dev/issue/27505]: https://go.dev/issue/27505
func Command(args ...string) *exec.Cmd {
	return command(args...)
}

// CommandContext is like [Command] but includes a context. It uses
// [exec.CommandContext] under the hood.
//
// The provided context is used to interrupt the process
// (by calling cmd.Cancel or [os.Process.Kill])
// if the context becomes done before the command completes on its own.
//
// CommandContext sets the command's Cancel function to invoke the Kill method
// on its Process, and leaves its WaitDelay unset. The caller may change the
// cancellation behavior by modifying those fields before starting the command.
func CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	return commandContext(ctx, args...)
}

// Self returns the executable path of the current process.
//
// On Linux, it returns "/proc/self/exe", which references the in-memory image
// of the running binary. This allows the on-disk binary (os.Args[0]) to be
// replaced or deleted without affecting re-execution.
//
// On other platforms, it attempts to resolve os.Args[0] to an absolute path.
// If resolution fails, it returns os.Args[0] unchanged.
func Self() string {
	if argv0, ok := reexecoverride.Argv0(); ok {
		return naiveSelf(argv0)
	}
	if runtime.GOOS == "linux" {
		return "/proc/self/exe"
	}
	return naiveSelf(os.Args[0])
}

// naiveSelf is a separate function to allow testing in isolation on Linux.
func naiveSelf(argv0 string) string {
	name := argv0
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
