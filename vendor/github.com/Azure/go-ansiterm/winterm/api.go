//go:build windows

package winterm

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ===========================================================================================================
// IMPORTANT NOTE:
//
//	The methods below make extensive use of the "unsafe" package to obtain the required pointers.
//	Beginning in Go 1.3, the garbage collector may release local variables (e.g., incoming arguments, stack
//	variables) the pointers reference *before* the API completes.
//
//  As a result, in those cases, the code must hint that the variables remain in active by invoking the
//	dummy method "use" (see below). Newer versions of Go are planned to change the mechanism to no longer
//	require unsafe pointers.
//
//	If you add or modify methods, ENSURE protection of local variables through the "use" builtin to inform
//	the garbage collector the variables remain in use if:
//
//	-- The value is not a pointer (e.g., int32, struct)
//	-- The value is not referenced by the method after passing the pointer to Windows
//
//	See http://golang.org/doc/go1.3.
// ===========================================================================================================

var (
	kernel32DLL = windows.NewLazyDLL("kernel32.dll")

	getConsoleCursorInfoProc       = kernel32DLL.NewProc("GetConsoleCursorInfo")
	setConsoleCursorInfoProc       = kernel32DLL.NewProc("SetConsoleCursorInfo")
	setConsoleScreenBufferSizeProc = kernel32DLL.NewProc("SetConsoleScreenBufferSize")
	scrollConsoleScreenBufferProc  = kernel32DLL.NewProc("ScrollConsoleScreenBufferA")
	setConsoleTextAttributeProc    = kernel32DLL.NewProc("SetConsoleTextAttribute")
	setConsoleWindowInfoProc       = kernel32DLL.NewProc("SetConsoleWindowInfo")
	writeConsoleOutputProc         = kernel32DLL.NewProc("WriteConsoleOutputW")
	readConsoleInputProc           = kernel32DLL.NewProc("ReadConsoleInputW")
)

// Console modes
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms686033(v=vs.85).aspx.
const (
	ENABLE_PROCESSED_INPUT        = windows.ENABLE_PROCESSED_INPUT
	ENABLE_LINE_INPUT             = windows.ENABLE_LINE_INPUT
	ENABLE_ECHO_INPUT             = windows.ENABLE_ECHO_INPUT
	ENABLE_WINDOW_INPUT           = windows.ENABLE_WINDOW_INPUT
	ENABLE_MOUSE_INPUT            = windows.ENABLE_MOUSE_INPUT
	ENABLE_INSERT_MODE            = windows.ENABLE_INSERT_MODE
	ENABLE_QUICK_EDIT_MODE        = windows.ENABLE_QUICK_EDIT_MODE
	ENABLE_EXTENDED_FLAGS         = windows.ENABLE_EXTENDED_FLAGS
	ENABLE_AUTO_POSITION          = windows.ENABLE_AUTO_POSITION
	ENABLE_VIRTUAL_TERMINAL_INPUT = windows.ENABLE_VIRTUAL_TERMINAL_INPUT

	ENABLE_PROCESSED_OUTPUT            = windows.ENABLE_PROCESSED_OUTPUT
	ENABLE_WRAP_AT_EOL_OUTPUT          = windows.ENABLE_WRAP_AT_EOL_OUTPUT
	ENABLE_VIRTUAL_TERMINAL_PROCESSING = windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	DISABLE_NEWLINE_AUTO_RETURN        = windows.DISABLE_NEWLINE_AUTO_RETURN
	ENABLE_LVB_GRID_WORLDWIDE          = windows.ENABLE_LVB_GRID_WORLDWIDE
)

// Character attributes
//
// Foreground and background attributes can be combined to produce colors.
// Clearing all foreground or background color bits produces black; setting
// all color bits produces white.
//
// See https://learn.microsoft.com/windows/console/char-info-str.
const (
	FOREGROUND_BLUE      uint16 = 0x0001
	FOREGROUND_GREEN     uint16 = 0x0002
	FOREGROUND_RED       uint16 = 0x0004
	FOREGROUND_INTENSITY uint16 = 0x0008
	FOREGROUND_MASK      uint16 = 0x000F

	BACKGROUND_BLUE      uint16 = 0x0010
	BACKGROUND_GREEN     uint16 = 0x0020
	BACKGROUND_RED       uint16 = 0x0040
	BACKGROUND_INTENSITY uint16 = 0x0080
	BACKGROUND_MASK      uint16 = 0x00F0

	COMMON_LVB_MASK          uint16 = 0xFF00
	COMMON_LVB_REVERSE_VIDEO uint16 = 0x4000
	COMMON_LVB_UNDERSCORE    uint16 = 0x8000
)

// Input event types
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms683499(v=vs.85).aspx.
const (
	KEY_EVENT                = windows.KEY_EVENT
	MOUSE_EVENT              = windows.MOUSE_EVENT
	WINDOW_BUFFER_SIZE_EVENT = windows.WINDOW_BUFFER_SIZE_EVENT
	MENU_EVENT               = windows.MENU_EVENT
	FOCUS_EVENT              = windows.FOCUS_EVENT
)

// WaitForSingleObject return codes.
const (
	WAIT_ABANDONED = windows.WAIT_ABANDONED
	WAIT_FAILED    = windows.WAIT_FAILED
	WAIT_SIGNALED  = windows.WAIT_OBJECT_0
	WAIT_TIMEOUT   = 0x00000102 // Keep as untyped int for backward compat; windows.WAIT_TIMEOUT is typed as syscall.Errno.
)

// WaitForSingleObject wait durations.
const (
	WAIT_INFINITE       = windows.INFINITE
	WAIT_ONE_SECOND     = 1000
	WAIT_HALF_SECOND    = 500
	WAIT_QUARTER_SECOND = 250
)

// Windows API Console types
// -- See https://msdn.microsoft.com/en-us/library/windows/desktop/ms682101(v=vs.85).aspx for Console specific types (e.g., COORD)
// -- See https://msdn.microsoft.com/en-us/library/aa296569(v=vs.60).aspx for comments on alignment
type (
	CHAR_INFO struct {
		UnicodeChar uint16
		Attributes  uint16
	}

	CONSOLE_CURSOR_INFO struct {
		Size    uint32
		Visible int32
	}

	// CONSOLE_SCREEN_BUFFER_INFO is identical to [windows.ConsoleScreenBufferInfo]
	// but implements [fmt.Stringer].
	CONSOLE_SCREEN_BUFFER_INFO struct {
		Size              COORD
		CursorPosition    COORD
		Attributes        uint16
		Window            SMALL_RECT
		MaximumWindowSize COORD
	}

	// COORD is identical to [windows.Coord] but implements [fmt.Stringer].
	COORD windows.Coord

	// SMALL_RECT is identical to [windows.SmallRect] but implements [fmt.Stringer].
	SMALL_RECT windows.SmallRect

	// INPUT_RECORD is a C/C++ union of which KEY_EVENT_RECORD is one case, it is also the largest
	// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms683499(v=vs.85).aspx.
	INPUT_RECORD struct {
		EventType uint16
		KeyEvent  KEY_EVENT_RECORD
	}

	KEY_EVENT_RECORD struct {
		KeyDown         int32
		RepeatCount     uint16
		VirtualKeyCode  uint16
		VirtualScanCode uint16
		UnicodeChar     uint16
		ControlKeyState uint32
	}

	WINDOW_BUFFER_SIZE struct {
		Size COORD
	}
)

// boolToBOOL converts a Go bool into a Windows int32.
func boolToBOOL(f bool) int32 {
	if f {
		return int32(1)
	} else {
		return int32(0)
	}
}

// GetConsoleCursorInfo retrieves information about the size and visiblity of the console cursor.
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms683163(v=vs.85).aspx.
func GetConsoleCursorInfo(handle uintptr, cursorInfo *CONSOLE_CURSOR_INFO) error {
	r1, r2, err := getConsoleCursorInfoProc.Call(handle, uintptr(unsafe.Pointer(cursorInfo)), 0)
	return checkError(r1, r2, err)
}

// SetConsoleCursorInfo sets the size and visiblity of the console cursor.
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms686019(v=vs.85).aspx.
func SetConsoleCursorInfo(handle uintptr, cursorInfo *CONSOLE_CURSOR_INFO) error {
	r1, r2, err := setConsoleCursorInfoProc.Call(handle, uintptr(unsafe.Pointer(cursorInfo)), 0)
	return checkError(r1, r2, err)
}

// SetConsoleCursorPosition location of the console cursor.
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms686025(v=vs.85).aspx.
func SetConsoleCursorPosition(handle uintptr, coord COORD) error {
	return windows.SetConsoleCursorPosition(windows.Handle(handle), windows.Coord(coord))
}

// GetConsoleMode gets the console mode for the given handle.
// See http://msdn.microsoft.com/en-us/library/windows/desktop/ms683167(v=vs.85).aspx.
func GetConsoleMode(handle uintptr) (mode uint32, err error) {
	err = windows.GetConsoleMode(windows.Handle(handle), &mode)
	return mode, err
}

// SetConsoleMode sets the console mode for the given handle.
// See http://msdn.microsoft.com/en-us/library/windows/desktop/ms686033(v=vs.85).aspx.
func SetConsoleMode(handle uintptr, mode uint32) error {
	return windows.SetConsoleMode(windows.Handle(handle), mode)
}

// GetConsoleScreenBufferInfo retrieves information about the specified console screen buffer.
// See http://msdn.microsoft.com/en-us/library/windows/desktop/ms683171(v=vs.85).aspx.
func GetConsoleScreenBufferInfo(handle uintptr) (*CONSOLE_SCREEN_BUFFER_INFO, error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(handle), &info); err != nil {
		return nil, err
	}

	return &CONSOLE_SCREEN_BUFFER_INFO{
		Size:              COORD(info.Size),
		CursorPosition:    COORD(info.CursorPosition),
		Attributes:        info.Attributes,
		Window:            SMALL_RECT(info.Window),
		MaximumWindowSize: COORD(info.MaximumWindowSize),
	}, nil
}

func ScrollConsoleScreenBuffer(handle uintptr, scrollRect SMALL_RECT, clipRect SMALL_RECT, destOrigin COORD, char CHAR_INFO) error {
	r1, r2, err := scrollConsoleScreenBufferProc.Call(handle, uintptr(unsafe.Pointer(&scrollRect)), uintptr(unsafe.Pointer(&clipRect)), coordToPointer(destOrigin), uintptr(unsafe.Pointer(&char)))
	runtime.KeepAlive(scrollRect)
	runtime.KeepAlive(clipRect)
	runtime.KeepAlive(destOrigin)
	runtime.KeepAlive(char)
	return checkError(r1, r2, err)
}

// SetConsoleScreenBufferSize sets the size of the console screen buffer.
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms686044(v=vs.85).aspx.
func SetConsoleScreenBufferSize(handle uintptr, coord COORD) error {
	r1, r2, err := setConsoleScreenBufferSizeProc.Call(handle, coordToPointer(coord))
	runtime.KeepAlive(coord)
	return checkError(r1, r2, err)
}

// SetConsoleTextAttribute sets the attributes of characters written to the
// console screen buffer by the WriteFile or WriteConsole function.
// See http://msdn.microsoft.com/en-us/library/windows/desktop/ms686047(v=vs.85).aspx.
func SetConsoleTextAttribute(handle uintptr, attribute uint16) error {
	r1, r2, err := setConsoleTextAttributeProc.Call(handle, uintptr(attribute), 0)
	runtime.KeepAlive(attribute)
	return checkError(r1, r2, err)
}

// SetConsoleWindowInfo sets the size and position of the console screen buffer's window.
// Note that the size and location must be within and no larger than the backing console screen buffer.
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms686125(v=vs.85).aspx.
func SetConsoleWindowInfo(handle uintptr, isAbsolute bool, rect SMALL_RECT) error {
	r1, r2, err := setConsoleWindowInfoProc.Call(handle, uintptr(boolToBOOL(isAbsolute)), uintptr(unsafe.Pointer(&rect)))
	runtime.KeepAlive(isAbsolute)
	runtime.KeepAlive(rect)
	return checkError(r1, r2, err)
}

// WriteConsoleOutput writes the CHAR_INFOs from the provided buffer to the active console buffer.
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms687404(v=vs.85).aspx.
func WriteConsoleOutput(handle uintptr, buffer []CHAR_INFO, bufferSize COORD, bufferCoord COORD, writeRegion *SMALL_RECT) error {
	r1, r2, err := writeConsoleOutputProc.Call(handle, uintptr(unsafe.Pointer(&buffer[0])), coordToPointer(bufferSize), coordToPointer(bufferCoord), uintptr(unsafe.Pointer(writeRegion)))
	runtime.KeepAlive(buffer)
	runtime.KeepAlive(bufferSize)
	runtime.KeepAlive(bufferCoord)
	return checkError(r1, r2, err)
}

// ReadConsoleInput reads (and removes) data from the console input buffer.
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms684961(v=vs.85).aspx.
func ReadConsoleInput(handle uintptr, buffer []INPUT_RECORD, count *uint32) error {
	r1, r2, err := readConsoleInputProc.Call(handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(count)))
	runtime.KeepAlive(buffer)
	return checkError(r1, r2, err)
}

// WaitForSingleObject waits for the passed handle to be signaled.
// It returns true if the handle was signaled; false otherwise.
// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms687032(v=vs.85).aspx.
func WaitForSingleObject(handle uintptr, msWait uint32) (bool, error) {
	event, err := windows.WaitForSingleObject(windows.Handle(handle), msWait)
	if err != nil {
		return false, err
	}

	switch event {
	case windows.WAIT_ABANDONED, uint32(windows.WAIT_TIMEOUT):
		return false, nil
	case windows.WAIT_OBJECT_0:
		return true, nil
	default:
		return false, nil
	}
}

// String helpers
func (info CONSOLE_SCREEN_BUFFER_INFO) String() string {
	return fmt.Sprintf("Size(%v) Cursor(%v) Window(%v) Max(%v)", info.Size, info.CursorPosition, info.Window, info.MaximumWindowSize)
}

func (coord COORD) String() string {
	return fmt.Sprintf("%v,%v", coord.X, coord.Y)
}

func (rect SMALL_RECT) String() string {
	return fmt.Sprintf("(%v,%v),(%v,%v)", rect.Left, rect.Top, rect.Right, rect.Bottom)
}

// checkError evaluates the results of a Windows API call and returns the error if it failed.
func checkError(r1, r2 uintptr, err error) error {
	// Windows APIs return non-zero to indicate success
	if r1 != 0 {
		return nil
	}

	// Return the error if provided, otherwise default to EINVAL
	if err != nil {
		return err
	}
	return syscall.EINVAL
}

// coordToPointer converts a COORD into a uintptr (by fooling the type system).
func coordToPointer(c COORD) uintptr {
	// Note: This code assumes the two SHORTs are correctly laid out; the "cast" to uint32 is just to get a pointer to pass.
	return uintptr(*((*uint32)(unsafe.Pointer(&c))))
}
