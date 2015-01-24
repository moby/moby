// +build windows

package term

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

// GetWinsize gets the window size of the given terminal
func GetWinsize(fd uintptr) (*Winsize, error) {
	ws := &Winsize{}
	var info *CONSOLE_SCREEN_BUFFER_INFO
	info, err := GetConsoleScreenBufferInfo(fd)
	if err != nil {
		return nil, err
	}

	ws.Width = uint16(info.Window.Right - info.Window.Left + 1)
	ws.Height = uint16(info.Window.Bottom - info.Window.Top + 1)

	ws.x = 0 // todo azlinux -- this is the pixel size of the Window, and not currently used by any caller
	ws.y = 0

	return ws, nil
}

// SetWinsize sets the terminal connected to the given file descriptor to a
// given size.
func SetWinsize(fd uintptr, ws *Winsize) error {
	return nil
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	_, e := GetConsoleMode(fd)
	return e == nil
}

// RestoreTerminal restores the terminal connected to the given file descriptor to a
// previous state.
func RestoreTerminal(fd uintptr, state *State) error {
	return SetConsoleMode(fd, state.mode)
}

// SaveState saves the state of the given console
func SaveState(fd uintptr) (*State, error) {
	mode, e := GetConsoleMode(fd)
	if e != nil {
		return nil, e
	}
	return &State{mode}, nil
}

// DisableEcho disbales the echo for given file descriptor and returns previous state
// see http://msdn.microsoft.com/en-us/library/windows/desktop/ms683462(v=vs.85).aspx for these flag settings
func DisableEcho(fd uintptr, state *State) error {
	state.mode &^= (ENABLE_ECHO_INPUT)
	state.mode |= (ENABLE_PROCESSED_INPUT | ENABLE_LINE_INPUT)
	return SetConsoleMode(fd, state.mode)
}

// SetRawTerminal puts the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func SetRawTerminal(fd uintptr) (*State, error) {
	oldState, err := MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	// TODO (azlinux): implement handling interrupt and restore state of terminal
	return oldState, err
}

// MakeRaw puts the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd uintptr) (*State, error) {
	var state *State
	state, err := SaveState(fd)
	if err != nil {
		return nil, err
	}

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms683462(v=vs.85).aspx
	// All three input modes, along with processed output mode, are designed to work together.
	// It is best to either enable or disable all of these modes as a group.
	// When all are enabled, the application is said to be in "cooked" mode, which means that most of the processing is handled for the application.
	// When all are disabled, the application is in "raw" mode, which means that input is unfiltered and any processing is left to the application.
	state.mode = 0
	err = SetConsoleMode(fd, state.mode)
	if err != nil {
		return nil, err
	}
	return state, nil
}
