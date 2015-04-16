// +build windows

package windows

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Windows keyboard constants
// See https://msdn.microsoft.com/en-us/library/windows/desktop/dd375731(v=vs.85).aspx.
const (
	VK_PRIOR    = 0x21 // PAGE UP key
	VK_NEXT     = 0x22 // PAGE DOWN key
	VK_END      = 0x23 // END key
	VK_HOME     = 0x24 // HOME key
	VK_LEFT     = 0x25 // LEFT ARROW key
	VK_UP       = 0x26 // UP ARROW key
	VK_RIGHT    = 0x27 // RIGHT ARROW key
	VK_DOWN     = 0x28 // DOWN ARROW key
	VK_SELECT   = 0x29 // SELECT key
	VK_PRINT    = 0x2A // PRINT key
	VK_EXECUTE  = 0x2B // EXECUTE key
	VK_SNAPSHOT = 0x2C // PRINT SCREEN key
	VK_INSERT   = 0x2D // INS key
	VK_DELETE   = 0x2E // DEL key
	VK_HELP     = 0x2F // HELP key
	VK_F1       = 0x70 // F1 key
	VK_F2       = 0x71 // F2 key
	VK_F3       = 0x72 // F3 key
	VK_F4       = 0x73 // F4 key
	VK_F5       = 0x74 // F5 key
	VK_F6       = 0x75 // F6 key
	VK_F7       = 0x76 // F7 key
	VK_F8       = 0x77 // F8 key
	VK_F9       = 0x78 // F9 key
	VK_F10      = 0x79 // F10 key
	VK_F11      = 0x7A // F11 key
	VK_F12      = 0x7B // F12 key

	RIGHT_ALT_PRESSED  = 0x0001
	LEFT_ALT_PRESSED   = 0x0002
	RIGHT_CTRL_PRESSED = 0x0004
	LEFT_CTRL_PRESSED  = 0x0008
	SHIFT_PRESSED      = 0x0010
	NUMLOCK_ON         = 0x0020
	SCROLLLOCK_ON      = 0x0040
	CAPSLOCK_ON        = 0x0080
	ENHANCED_KEY       = 0x0100
)

// ANSI constants
// References:
// -- http://www.ecma-international.org/publications/standards/Ecma-048.htm
// -- http://man7.org/linux/man-pages/man4/console_codes.4.html
// -- http://manpages.ubuntu.com/manpages/intrepid/man4/console_codes.4.html
// -- https://en.wikipedia.org/wiki/ANSI_escape_code
// -- http://vt100.net/emu/dec_ansi_parser
// -- http://vt100.net/emu/vt500_parser.svg
// -- http://invisible-island.net/xterm/ctlseqs/ctlseqs.html
// -- https://www.inwap.com/pdp10/ansicode.txt
const (
	// ECMA-48 Set Graphics Rendition
	// Note:
	// -- Constants leading with an underscore (e.g., _ANSI_xxx) are unsupported or reserved
	// -- Fonts could possibly be supported via SetCurrentConsoleFontEx
	// -- Windows does not expose the per-window cursor (i.e., caret) blink times
	ANSI_SGR_RESET              = 0
	ANSI_SGR_BOLD               = 1
	ANSI_SGR_DIM                = 2
	_ANSI_SGR_ITALIC            = 3
	ANSI_SGR_UNDERLINE          = 4
	_ANSI_SGR_BLINKSLOW         = 5
	_ANSI_SGR_BLINKFAST         = 6
	ANSI_SGR_REVERSE            = 7
	_ANSI_SGR_INVISIBLE         = 8
	_ANSI_SGR_LINETHROUGH       = 9
	_ANSI_SGR_FONT_00           = 10
	_ANSI_SGR_FONT_01           = 11
	_ANSI_SGR_FONT_02           = 12
	_ANSI_SGR_FONT_03           = 13
	_ANSI_SGR_FONT_04           = 14
	_ANSI_SGR_FONT_05           = 15
	_ANSI_SGR_FONT_06           = 16
	_ANSI_SGR_FONT_07           = 17
	_ANSI_SGR_FONT_08           = 18
	_ANSI_SGR_FONT_09           = 19
	_ANSI_SGR_FONT_10           = 20
	_ANSI_SGR_DOUBLEUNDERLINE   = 21
	ANSI_SGR_BOLD_DIM_OFF       = 22
	_ANSI_SGR_ITALIC_OFF        = 23
	ANSI_SGR_UNDERLINE_OFF      = 24
	_ANSI_SGR_BLINK_OFF         = 25
	_ANSI_SGR_RESERVED_00       = 26
	ANSI_SGR_REVERSE_OFF        = 27
	_ANSI_SGR_INVISIBLE_OFF     = 28
	_ANSI_SGR_LINETHROUGH_OFF   = 29
	ANSI_SGR_FOREGROUND_BLACK   = 30
	ANSI_SGR_FOREGROUND_RED     = 31
	ANSI_SGR_FOREGROUND_GREEN   = 32
	ANSI_SGR_FOREGROUND_YELLOW  = 33
	ANSI_SGR_FOREGROUND_BLUE    = 34
	ANSI_SGR_FOREGROUND_MAGENTA = 35
	ANSI_SGR_FOREGROUND_CYAN    = 36
	ANSI_SGR_FOREGROUND_WHITE   = 37
	_ANSI_SGR_RESERVED_01       = 38
	ANSI_SGR_FOREGROUND_DEFAULT = 39
	ANSI_SGR_BACKGROUND_BLACK   = 40
	ANSI_SGR_BACKGROUND_RED     = 41
	ANSI_SGR_BACKGROUND_GREEN   = 42
	ANSI_SGR_BACKGROUND_YELLOW  = 43
	ANSI_SGR_BACKGROUND_BLUE    = 44
	ANSI_SGR_BACKGROUND_MAGENTA = 45
	ANSI_SGR_BACKGROUND_CYAN    = 46
	ANSI_SGR_BACKGROUND_WHITE   = 47
	_ANSI_SGR_RESERVED_02       = 48
	ANSI_SGR_BACKGROUND_DEFAULT = 49
	// 50 - 65: Unsupported

	ANSI_MAX_CMD_LENGTH = 256

	MAX_INPUT_EVENTS = 128
	DEFAULT_WIDTH    = 80
	DEFAULT_HEIGHT   = 24

	ANSI_ESCAPE_PRIMARY   = 0x1B
	ANSI_ESCAPE_SECONDARY = 0x5B
	ANSI_COMMAND_FIRST    = 0x40
	ANSI_COMMAND_LAST     = 0x7E
	ANSI_PARAMETER_SEP    = ";"
	ANSI_CMD_G0           = '('
	ANSI_CMD_G1           = ')'
	ANSI_CMD_G2           = '*'
	ANSI_CMD_G3           = '+'
	ANSI_CMD_DECPNM       = '>'
	ANSI_CMD_DECPAM       = '='
	ANSI_CMD_OSC          = ']'
	ANSI_CMD_STR_TERM     = '\\'
	ANSI_BEL              = 0x07

	KEY_CONTROL_PARAM_2 = ";2"
	KEY_CONTROL_PARAM_3 = ";3"
	KEY_CONTROL_PARAM_4 = ";4"
	KEY_CONTROL_PARAM_5 = ";5"
	KEY_CONTROL_PARAM_6 = ";6"
	KEY_CONTROL_PARAM_7 = ";7"
	KEY_CONTROL_PARAM_8 = ";8"
	KEY_ESC_CSI         = "\x1B["
	KEY_ESC_N           = "\x1BN"
	KEY_ESC_O           = "\x1BO"

	FILL_CHARACTER = ' '
)

type ansiCommand struct {
	CommandBytes []byte
	Command      string
	Parameters   []string
	IsSpecial    bool
}

func newAnsiCommand(command []byte) *ansiCommand {

	if isCharacterSelectionCmdChar(command[1]) {
		// Is Character Set Selection commands
		return &ansiCommand{
			CommandBytes: command,
			Command:      string(command),
			IsSpecial:    true,
		}
	}

	// last char is command character
	lastCharIndex := len(command) - 1

	ac := &ansiCommand{
		CommandBytes: command,
		Command:      string(command[lastCharIndex]),
		IsSpecial:    false,
	}

	// more than a single escape
	if lastCharIndex != 0 {
		start := 1
		// skip if double char escape sequence
		if command[0] == ANSI_ESCAPE_PRIMARY && command[1] == ANSI_ESCAPE_SECONDARY {
			start++
		}
		// convert this to GetNextParam method
		ac.Parameters = strings.Split(string(command[start:lastCharIndex]), ANSI_PARAMETER_SEP)
	}

	return ac
}

func (ac *ansiCommand) param(index int) string {
	if index < 0 || index >= len(ac.Parameters) {
		return ""
	}
	return ac.Parameters[index]
}

func (ac *ansiCommand) paramAsSHORT(index int, defaultValue SHORT) SHORT {
	s := ac.param(index)
	if s == "" {
		return defaultValue
	}
	n, err := strconv.ParseInt(s, 10, 16)
	if err != nil {
		return defaultValue
	}
	return SHORT(n)
}

func (ac *ansiCommand) String() string {
	return fmt.Sprintf("0x%v \"%v\" (\"%v\")",
		bytesToHex(ac.CommandBytes),
		ac.Command,
		strings.Join(ac.Parameters, "\",\""))
}

// isAnsiCommandChar returns true if the passed byte falls within the range of ANSI commands.
// See http://manpages.ubuntu.com/manpages/intrepid/man4/console_codes.4.html.
func isAnsiCommandChar(b byte) bool {
	switch {
	case ANSI_COMMAND_FIRST <= b && b <= ANSI_COMMAND_LAST && b != ANSI_ESCAPE_SECONDARY:
		return true
	case b == ANSI_CMD_G1 || b == ANSI_CMD_OSC || b == ANSI_CMD_DECPAM || b == ANSI_CMD_DECPNM:
		// non-CSI escape sequence terminator
		return true
	case b == ANSI_CMD_STR_TERM || b == ANSI_BEL:
		// String escape sequence terminator
		return true
	}
	return false
}

func isXtermOscSequence(command []byte, current byte) bool {
	return (len(command) >= 2 && command[0] == ANSI_ESCAPE_PRIMARY && command[1] == ANSI_CMD_OSC && current != ANSI_BEL)
}

func isCharacterSelectionCmdChar(b byte) bool {
	return (b == ANSI_CMD_G0 || b == ANSI_CMD_G1 || b == ANSI_CMD_G2 || b == ANSI_CMD_G3)
}

// addInRange increments a value by the passed quantity while ensuring the values
// always remain (without overflow) within the supplied min / max range.
func addInRange(n SHORT, increment SHORT, min SHORT, max SHORT) SHORT {
	n += increment
	if increment >= 0 && (n < 0 || n > max) {
		return max
	} else if increment < 0 && (n > 0 || n < min) {
		return min
	} else {
		return n
	}
}

// bytesToHex converts a slice of bytes to a human-readable string.
func bytesToHex(b []byte) string {
	hex := make([]string, len(b))
	for i, ch := range b {
		hex[i] = fmt.Sprintf("%X", ch)
	}
	return strings.Join(hex, "")
}

// collectAnsiIntoWindowsAttributes modifies the passed Windows text mode flags to reflect the
// request represented by the passed ANSI mode.
func collectAnsiIntoWindowsAttributes(windowsMode WORD, baseMode WORD, ansiMode SHORT) WORD {
	switch ansiMode {

	// Mode styles
	case ANSI_SGR_BOLD:
		windowsMode = windowsMode | FOREGROUND_INTENSITY

	case ANSI_SGR_DIM, ANSI_SGR_BOLD_DIM_OFF:
		windowsMode &^= FOREGROUND_INTENSITY

	case ANSI_SGR_UNDERLINE:
		windowsMode = windowsMode | COMMON_LVB_UNDERSCORE

	case ANSI_SGR_REVERSE, ANSI_SGR_REVERSE_OFF:
		// Note: Windows does not support a native reverse. Simply swap the foreground / background color / intensity.
		windowsMode = (COMMON_LVB_MASK & windowsMode) | ((FOREGROUND_MASK & windowsMode) << 4) | ((BACKGROUND_MASK & windowsMode) >> 4)

	case ANSI_SGR_UNDERLINE_OFF:
		windowsMode &^= COMMON_LVB_UNDERSCORE

	// Foreground colors
	case ANSI_SGR_FOREGROUND_DEFAULT:
		windowsMode = (windowsMode & ^FOREGROUND_MASK) | (baseMode & FOREGROUND_MASK)

	case ANSI_SGR_FOREGROUND_BLACK:
		windowsMode = windowsMode ^ (FOREGROUND_RED | FOREGROUND_GREEN | FOREGROUND_BLUE)

	case ANSI_SGR_FOREGROUND_RED:
		windowsMode = (windowsMode & ^FOREGROUND_MASK) | FOREGROUND_RED

	case ANSI_SGR_FOREGROUND_GREEN:
		windowsMode = (windowsMode & ^FOREGROUND_MASK) | FOREGROUND_GREEN

	case ANSI_SGR_FOREGROUND_YELLOW:
		windowsMode = (windowsMode & ^FOREGROUND_MASK) | FOREGROUND_RED | FOREGROUND_GREEN

	case ANSI_SGR_FOREGROUND_BLUE:
		windowsMode = (windowsMode & ^FOREGROUND_MASK) | FOREGROUND_BLUE

	case ANSI_SGR_FOREGROUND_MAGENTA:
		windowsMode = (windowsMode & ^FOREGROUND_MASK) | FOREGROUND_RED | FOREGROUND_BLUE

	case ANSI_SGR_FOREGROUND_CYAN:
		windowsMode = (windowsMode & ^FOREGROUND_MASK) | FOREGROUND_GREEN | FOREGROUND_BLUE

	case ANSI_SGR_FOREGROUND_WHITE:
		windowsMode = (windowsMode & ^FOREGROUND_MASK) | FOREGROUND_RED | FOREGROUND_GREEN | FOREGROUND_BLUE

	// Background colors
	case ANSI_SGR_BACKGROUND_DEFAULT:
		// Black with no intensity
		windowsMode = (windowsMode & ^BACKGROUND_MASK) | (baseMode & BACKGROUND_MASK)

	case ANSI_SGR_BACKGROUND_BLACK:
		windowsMode = (windowsMode & ^BACKGROUND_MASK)

	case ANSI_SGR_BACKGROUND_RED:
		windowsMode = (windowsMode & ^BACKGROUND_MASK) | BACKGROUND_RED

	case ANSI_SGR_BACKGROUND_GREEN:
		windowsMode = (windowsMode & ^BACKGROUND_MASK) | BACKGROUND_GREEN

	case ANSI_SGR_BACKGROUND_YELLOW:
		windowsMode = (windowsMode & ^BACKGROUND_MASK) | BACKGROUND_RED | BACKGROUND_GREEN

	case ANSI_SGR_BACKGROUND_BLUE:
		windowsMode = (windowsMode & ^BACKGROUND_MASK) | BACKGROUND_BLUE

	case ANSI_SGR_BACKGROUND_MAGENTA:
		windowsMode = (windowsMode & ^BACKGROUND_MASK) | BACKGROUND_RED | BACKGROUND_BLUE

	case ANSI_SGR_BACKGROUND_CYAN:
		windowsMode = (windowsMode & ^BACKGROUND_MASK) | BACKGROUND_GREEN | BACKGROUND_BLUE

	case ANSI_SGR_BACKGROUND_WHITE:
		windowsMode = (windowsMode & ^BACKGROUND_MASK) | BACKGROUND_RED | BACKGROUND_GREEN | BACKGROUND_BLUE
	}

	return windowsMode
}

// ensureInRange adjusts the passed value, if necessary, to ensure it is within
// the passed min / max range.
func ensureInRange(n SHORT, min SHORT, max SHORT) SHORT {
	if n < min {
		return min
	} else if n > max {
		return max
	} else {
		return n
	}
}

func getStdFile(nFile int) (*os.File, uintptr) {
	var file *os.File
	switch nFile {
	case syscall.STD_INPUT_HANDLE:
		file = os.Stdin
	case syscall.STD_OUTPUT_HANDLE:
		file = os.Stdout
	case syscall.STD_ERROR_HANDLE:
		file = os.Stderr
	default:
		panic(fmt.Errorf("Invalid standard handle identifier: %v", nFile))
	}

	fd, err := syscall.GetStdHandle(nFile)
	if err != nil {
		panic(fmt.Errorf("Invalid standard handle indentifier: %v -- %v", nFile, err))
	}

	return file, uintptr(fd)
}
