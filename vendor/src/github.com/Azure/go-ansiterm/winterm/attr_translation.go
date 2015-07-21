// +build windows

package winterm

import (
	. "github.com/Azure/go-ansiterm"
)

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
