// +build windows

package term

import (
	"io"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/term/winconsole"
)

// State holds the console mode for the terminal.
type State struct {
	mode uint32
}

// Winsize is used for window size.
type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16
	y      uint16
}

// StdStreams returns the standard streams (stdin, stdout, stedrr).
func StdStreams() (stdIn io.ReadCloser, stdOut, stdErr io.Writer) {
	switch {
	case os.Getenv("ConEmuANSI") == "ON":
		// The ConEmu shell emulates ANSI well by default.
		return os.Stdin, os.Stdout, os.Stderr
	case os.Getenv("MSYSTEM") != "":
		// MSYS (mingw) does not emulate ANSI well.
		return winconsole.WinConsoleStreams()
	default:
		return winconsole.WinConsoleStreams()
	}
}

// GetFdInfo returns the file descriptor for an os.File and indicates whether the file represents a terminal.
func GetFdInfo(in interface{}) (uintptr, bool) {
	return winconsole.GetHandleInfo(in)
}

// GetWinsize returns the window size based on the specified file descriptor.
func GetWinsize(fd uintptr) (*Winsize, error) {
	info, err := winconsole.GetConsoleScreenBufferInfo(fd)
	if err != nil {
		return nil, err
	}

	// TODO(azlinux): Set the pixel width / height of the console (currently unused by any caller)
	return &Winsize{
		Width:  uint16(info.Window.Right - info.Window.Left + 1),
		Height: uint16(info.Window.Bottom - info.Window.Top + 1),
		x:      0,
		y:      0}, nil
}

// SetWinsize tries to set the specified window size for the specified file descriptor.
func SetWinsize(fd uintptr, ws *Winsize) error {
	// TODO(azlinux): Implement SetWinsize
	logrus.Debugf("[windows] SetWinsize: WARNING -- Unsupported method invoked")
	return nil
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	return winconsole.IsConsole(fd)
}

// RestoreTerminal restores the terminal connected to the given file descriptor
// to a previous state.
func RestoreTerminal(fd uintptr, state *State) error {
	return winconsole.SetConsoleMode(fd, state.mode)
}

// SaveState saves the state of the terminal connected to the given file descriptor.
func SaveState(fd uintptr) (*State, error) {
	mode, e := winconsole.GetConsoleMode(fd)
	if e != nil {
		return nil, e
	}
	return &State{mode}, nil
}

// DisableEcho disables echo for the terminal connected to the given file descriptor.
// -- See http://msdn.microsoft.com/en-us/library/windows/desktop/ms683462(v=vs.85).aspx
func DisableEcho(fd uintptr, state *State) error {
	mode := state.mode
	mode &^= winconsole.ENABLE_ECHO_INPUT
	mode |= winconsole.ENABLE_PROCESSED_INPUT | winconsole.ENABLE_LINE_INPUT
	// TODO(azlinux): Core code registers a goroutine to catch os.Interrupt and reset the terminal state.
	return winconsole.SetConsoleMode(fd, mode)
}

// SetRawTerminal puts the terminal connected to the given file descriptor into raw
// mode and returns the previous state.
func SetRawTerminal(fd uintptr) (*State, error) {
	state, err := MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	// TODO(azlinux): Core code registers a goroutine to catch os.Interrupt and reset the terminal state.
	return state, err
}

// MakeRaw puts the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd uintptr) (*State, error) {
	state, err := SaveState(fd)
	if err != nil {
		return nil, err
	}

	// See
	// -- https://msdn.microsoft.com/en-us/library/windows/desktop/ms686033(v=vs.85).aspx
	// -- https://msdn.microsoft.com/en-us/library/windows/desktop/ms683462(v=vs.85).aspx
	mode := state.mode

	// Disable these modes
	mode &^= winconsole.ENABLE_ECHO_INPUT
	mode &^= winconsole.ENABLE_LINE_INPUT
	mode &^= winconsole.ENABLE_MOUSE_INPUT
	mode &^= winconsole.ENABLE_WINDOW_INPUT
	mode &^= winconsole.ENABLE_PROCESSED_INPUT

	// Enable these modes
	mode |= winconsole.ENABLE_EXTENDED_FLAGS
	mode |= winconsole.ENABLE_INSERT_MODE
	mode |= winconsole.ENABLE_QUICK_EDIT_MODE

	err = winconsole.SetConsoleMode(fd, mode)
	if err != nil {
		return nil, err
	}
	return state, nil
}
