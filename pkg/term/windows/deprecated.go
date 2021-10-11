//go:build windows
// +build windows

// Package windowsconsole implements ANSI-aware input and output streams for use
// by the Docker Windows client. When asked for the set of standard streams (e.g.,
// stdin, stdout, stderr), the code will create and return pseudo-streams that
// convert ANSI sequences to / from Windows Console API calls.
//
// Deprecated: use github.com/moby/term/windows instead
package windowsconsole // import "github.com/docker/docker/pkg/term/windows"

import (
	windowsconsole "github.com/moby/term/windows"
)

var (
	// GetHandleInfo returns file descriptor and bool indicating whether the file is a console.
	// Deprecated: use github.com/moby/term/windows.GetHandleInfo
	GetHandleInfo = windowsconsole.GetHandleInfo

	// IsConsole returns true if the given file descriptor is a Windows Console.
	// The code assumes that GetConsoleMode will return an error for file descriptors that are not a console.
	// Deprecated: use github.com/moby/term/windows.IsConsole
	IsConsole = windowsconsole.IsConsole

	// NewAnsiReader returns an io.ReadCloser that provides VT100 terminal emulation on top of a
	// Windows console input handle.
	// Deprecated: use github.com/moby/term/windows.NewAnsiReader
	NewAnsiReader = windowsconsole.NewAnsiReader

	// NewAnsiWriter returns an io.Writer that provides VT100 terminal emulation on top of a
	// Windows console output handle.
	// Deprecated: use github.com/moby/term/windows.NewAnsiWriter
	NewAnsiWriter = windowsconsole.NewAnsiWriter
)
