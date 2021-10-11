// Package signal provides helper functions for dealing with signals across
// various operating systems.
package signal // import "github.com/docker/docker/pkg/signal"

import (
	"github.com/docker/docker/pkg/stack"
	msignal "github.com/moby/sys/signal"
)

var (
	// DumpStacks appends the runtime stack into file in dir and returns full path
	// to that file.
	// Deprecated: use github.com/docker/docker/pkg/stack.Dump instead.
	DumpStacks = stack.DumpToFile

	// CatchAll catches all signals and relays them to the specified channel.
	// SIGURG is not handled, as it's used by the Go runtime to support
	// preemptable system calls.
	// Deprecated: use github.com/moby/sys/signal.CatchAll instead
	CatchAll = msignal.CatchAll

	// StopCatch stops catching the signals and closes the specified channel.
	// Deprecated: use github.com/moby/sys/signal.StopCatch instead
	StopCatch = msignal.StopCatch

	// ParseSignal translates a string to a valid syscall signal.
	// It returns an error if the signal map doesn't include the given signal.
	// Deprecated: use github.com/moby/sys/signal.ParseSignal instead
	ParseSignal = msignal.ParseSignal

	// ValidSignalForPlatform returns true if a signal is valid on the platform
	// Deprecated: use github.com/moby/sys/signal.ValidSignalForPlatform instead
	ValidSignalForPlatform = msignal.ValidSignalForPlatform

	// SignalMap is a map of signals for the current platform.
	// Deprecated: use github.com/moby/sys/signal.SignalMap instead
	SignalMap = msignal.SignalMap
)

// Signals used in cli/command
const (
	// SIGCHLD is a signal sent to a process when a child process terminates, is interrupted, or resumes after being interrupted.
	// Deprecated: use github.com/moby/sys/signal.SIGCHLD instead
	SIGCHLD = msignal.SIGCHLD
	// SIGWINCH is a signal sent to a process when its controlling terminal changes its size
	// Deprecated: use github.com/moby/sys/signal.SIGWINCH instead
	SIGWINCH = msignal.SIGWINCH
	// SIGPIPE is a signal sent to a process when a pipe is written to before the other end is open for reading
	// Deprecated: use github.com/moby/sys/signal.SIGPIPE instead
	SIGPIPE = msignal.SIGPIPE

	// DefaultStopSignal has been deprecated and removed. The default value is
	// now defined in github.com/docker/docker/container. Clients should omit
	// the container's stop-signal field if the default should be used.
)
