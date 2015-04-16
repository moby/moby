// +build windows

package windows

import (
	"io"
	"os"
	"syscall"
)

// ConsoleStreams, for each standard stream referencing a console, returns a wrapped
// version that handles ANSI character sequences.
func ConsoleStreams() (stdIn io.ReadCloser, stdOut, stdErr io.Writer) {
	if IsConsole(os.Stdin.Fd()) {
		stdIn = newAnsiReader(syscall.STD_INPUT_HANDLE)
	} else {
		stdIn = os.Stdin
	}

	if IsConsole(os.Stdout.Fd()) {
		stdOut = newAnsiWriter(syscall.STD_OUTPUT_HANDLE)
	} else {
		stdOut = os.Stdout
	}

	if IsConsole(os.Stderr.Fd()) {
		stdErr = newAnsiWriter(syscall.STD_ERROR_HANDLE)
	} else {
		stdErr = os.Stderr
	}

	return stdIn, stdOut, stdErr
}

// GetHandleInfo returns file descriptor and bool indicating whether the file is a console.
func GetHandleInfo(stream interface{}) (uintptr, bool) {
	switch stream := stream.(type) {
	case *ansiReader:
		return stream.Fd(), true
	case *ansiWriter:
		return stream.Fd(), true
	}

	var streamFd uintptr
	var isTerminal bool

	if file, ok := stream.(*os.File); ok {
		streamFd = file.Fd()
		isTerminal = IsConsole(streamFd)
	}
	return streamFd, isTerminal
}

// IsConsole returns true if the given file descriptor is a Windows Console.
// The code assumes that GetConsoleMode will return an error for file descriptors that are not a console.
func IsConsole(fd uintptr) bool {
	_, e := GetConsoleMode(fd)
	return e == nil
}
