// +build windows

package windows

import (
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
)

// ansiWriter wraps a standard output file (e.g., os.Stdout) providing ANSI sequence translation.
type ansiWriter struct {
	file           *os.File
	fd             uintptr
	infoReset      *CONSOLE_SCREEN_BUFFER_INFO
	command        []byte
	escapeSequence []byte
	inAnsiSequence bool
}

func newAnsiWriter(nFile int) *ansiWriter {
	file, fd := getStdFile(nFile)

	info, err := GetConsoleScreenBufferInfo(fd)
	if err != nil {
		return nil
	}

	return &ansiWriter{
		file:           file,
		fd:             fd,
		infoReset:      info,
		command:        make([]byte, 0, ANSI_MAX_CMD_LENGTH),
		escapeSequence: []byte(KEY_ESC_CSI),
	}
}

func (aw *ansiWriter) Fd() uintptr {
	return aw.fd
}

// Write writes len(p) bytes from p to the underlying data stream.
func (aw *ansiWriter) Write(p []byte) (total int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Iterate through the passed bytes converting ANSI commands and writing normal characters
	sequenceStart := 0
	for i, char := range p {

		// Accumulate the command characters and process once complete
		if aw.inAnsiSequence {
			aw.command = append(aw.command, char)

			if isAnsiCommandChar(char) && !isXtermOscSequence(aw.command, char) {
				aw.inAnsiSequence = false
				sequenceStart = i + 1
				total += len(aw.command)

				err := aw.doAnsiCommand()
				aw.command = aw.command[:0]
				if err != nil {
					break
				}
			}

			// Otherwise, search for the start of the next ANSI command
		} else {
			if char == ANSI_ESCAPE_PRIMARY {
				aw.inAnsiSequence = true
				aw.command = append(aw.command, char)

				// Write any non-ANSI characters encountered
				if sequenceStart < i {
					count, err := aw.file.Write(p[sequenceStart:i])
					total += count
					if err != nil {
						break
					}
				}
			}
		}
	}

	// If an error occurred, reject all remaining characters and reset ANSI command status
	if err != nil {
		aw.inAnsiSequence = false
		aw.command = aw.command[:0]
		return total, err
	}

	// If outside an ANSI sequence and bytes remain, write them
	// -- Discovery of an ANSI escape sequence, in the above loop, triggers writing
	//    non-ANSI characters; arriving here with remaining bytes means only non-ANSI
	//    characters remain to be written
	if !aw.inAnsiSequence && sequenceStart < len(p) {
		count, err := aw.file.Write(p[sequenceStart:])
		total += count
		if err != nil {
			return total, err
		}
	}

	return total, nil
}

func (aw *ansiWriter) clearRange(attributes WORD, fromCoord COORD, toCoord COORD) error {
	// Ignore an invalid (negative area) request
	if toCoord.Y < fromCoord.Y {
		return nil
	}

	var err error

	var coordStart = COORD{}
	var coordEnd = COORD{}

	xCurrent, yCurrent := fromCoord.X, fromCoord.Y
	xEnd, yEnd := toCoord.X, toCoord.Y

	// Clear any partial initial line
	if xCurrent > 0 {
		coordStart.X, coordStart.Y = xCurrent, yCurrent
		coordEnd.X, coordEnd.Y = xEnd, yCurrent

		err = aw.clearRect(attributes, coordStart, coordEnd)
		if err != nil {
			return err
		}

		xCurrent = 0
		yCurrent += 1
	}

	// Clear intervening rectangular section
	if yCurrent < yEnd {
		coordStart.X, coordStart.Y = xCurrent, yCurrent
		coordEnd.X, coordEnd.Y = xEnd, yEnd-1

		err = aw.clearRect(attributes, coordStart, coordEnd)
		if err != nil {
			return err
		}

		xCurrent = 0
		yCurrent = yEnd
	}

	// Clear remaining partial ending line
	coordStart.X, coordStart.Y = xCurrent, yCurrent
	coordEnd.X, coordEnd.Y = xEnd, yEnd

	err = aw.clearRect(attributes, coordStart, coordEnd)
	if err != nil {
		return err
	}

	return nil
}

func (aw *ansiWriter) clearRect(attributes WORD, fromCoord COORD, toCoord COORD) error {
	region := SMALL_RECT{Top: fromCoord.Y, Left: fromCoord.X, Bottom: toCoord.Y, Right: toCoord.X}

	width := toCoord.X - fromCoord.X + 1
	height := toCoord.Y - fromCoord.Y + 1
	size := uint32(width) * uint32(height)
	buffer := make([]CHAR_INFO, size)

	if size <= 0 {
		return nil
	}

	char := CHAR_INFO{WCHAR(FILL_CHARACTER), attributes}
	for i := 0; i < int(size); i++ {
		buffer[i] = char
	}

	err := WriteConsoleOutput(aw.fd, buffer, &COORD{X: width, Y: height}, &COORD{X: 0, Y: 0}, &region)
	if err != nil {
		return err
	}

	return nil
}

// doAnsiCommand translates ANSI commands into Windows API calls.
func (aw *ansiWriter) doAnsiCommand() (err error) {

	info, err := GetConsoleScreenBufferInfo(aw.fd)
	if err != nil {
		return err
	}

	ac := newAnsiCommand(aw.command)
	logrus.Debugf("[windows] doAnsiCommand: Cmd(%v)", ac)

	switch ac.Command {
	case "A", "B", "C", "D", "E", "F", "G":
		// [incrementA -- Move up
		// [incrementB -- Move down
		// [incrementC -- Move right
		// [incrementD -- Move left
		// [incrementE -- Move to (0, y+increment)
		// [incrementF -- Move to (0, y-increment)
		// [columnG -- Move to column (1-based) in current line

		value := ac.paramAsSHORT(0, 1)

		if ac.Command == "A" || ac.Command == "D" || ac.Command == "F" {
			value = -value
		}

		position := info.CursorPosition
		if ac.Command == "C" || ac.Command == "D" {
			position.X = addInRange(position.X, value, info.Window.Left, info.Window.Right)
		} else if ac.Command == "G" {
			position.X = addInRange(value, -1, info.Window.Left, info.Window.Right)
		} else {
			if ac.Command == "E" || ac.Command == "F" {
				position.X = 0
			}
			position.Y = addInRange(position.Y, value, info.Window.Top, info.Window.Bottom)
		}

		err = aw.setCursorPosition(&position, info.Size)
		if err != nil {
			return err
		}

	case "H", "f":
		// [row;columnH
		// [row;columnf
		// Move to the specified row and column (default is 1,1), pinned to the current window

		position := COORD{
			addInRange(ac.paramAsSHORT(1, 1), -1, info.Window.Left, info.Window.Right),
			addInRange(ac.paramAsSHORT(0, 1), -1, info.Window.Top, info.Window.Bottom)}

		err = aw.setCursorPosition(&position, info.Size)
		if err != nil {
			return err
		}

	case "h":
		// TODO(azlinux): Review this ANSI sequence handling
		for _, value := range ac.Parameters {
			switch value {
			case "?25", "25":
				aw.setCursorVisible(true)
			case "?1049", "1049":
				// TODO (azlinux): Save terminal
			case "?1", "1":
				// If the DECCKM function is set, then the arrow keys send application sequences to the host.
				// DECCKM (default off): When set, the cursor keys send an ESC O prefix, rather than ESC [.
				aw.escapeSequence = []byte(KEY_ESC_O)
			}
		}

	case "J":
		// [J  -- Erases from the cursor to the end of the screen, including the cursor position.
		// [1J -- Erases from the beginning of the screen to the cursor, including the cursor position.
		// [2J -- Erases the complete display. The cursor does not move.
		// [3J -- Erases the complete display and backing buffer, cursor moves to (0,0)
		// Notes:
		// -- ANSI.SYS always moved the cursor to (0,0) for both [2J and [3J
		// -- Clearing the entire buffer, versus just the Window, works best for Windows Consoles
		value := ac.paramAsSHORT(0, 0)

		var start COORD
		var end COORD

		switch value {
		case 0:
			start = info.CursorPosition
			end = COORD{info.Size.X - 1, info.Size.Y - 1}

		case 1:
			start = COORD{0, 0}
			end = info.CursorPosition

		case 2:
			start = COORD{0, 0}
			end = COORD{info.Size.X - 1, info.Size.Y - 1}

		case 3:
			start = COORD{0, 0}
			end = COORD{info.Size.X - 1, info.Size.Y - 1}
		}

		err = aw.clearRange(info.Attributes, start, end)
		if err != nil {
			return err
		}

		if value == 2 || value == 3 {
			err = aw.setCursorPosition(&COORD{0, 0}, info.Size)
			if err != nil {
				return err
			}
		}

	case "K":
		// [K  -- Erases from the cursor to the end of the line, including the cursor position.
		// [1K -- Erases from the beginning of the line to the cursor, including the cursor position.
		// [2K -- Erases the complete line.
		value := ac.paramAsSHORT(0, 0)

		var start COORD
		var end COORD

		switch value {
		case 0:
			start = info.CursorPosition
			end = COORD{info.Window.Right, info.CursorPosition.Y}

		case 1:
			start = COORD{0, info.CursorPosition.Y}
			end = info.CursorPosition

		case 2:
			start = COORD{0, info.CursorPosition.Y}
			end = COORD{info.Window.Right, info.CursorPosition.Y}
		}

		err = aw.clearRange(info.Attributes, start, end)
		if err != nil {
			return err
		}

	case "l":
		// TODO(azlinux): Review this ANSI sequence handling
		for _, value := range ac.Parameters {
			switch value {
			case "?25", "25":
				aw.setCursorVisible(false)
			case "?1049", "1049":
				// TODO (azlinux):  Restore terminal
			case "?1", "1":
				// If the DECCKM function is reset, then the arrow keys send ANSI cursor sequences to the host.
				aw.escapeSequence = []byte(KEY_ESC_CSI)
			}
		}

	case "m":
		// [value;...;valuem -- Set graphic rendition (SGR) mode as specified by the values; no values means a reset

		attributes := info.Attributes
		if len(ac.Parameters) <= 0 {
			attributes = aw.infoReset.Attributes
		} else {
			for _, e := range ac.Parameters {
				ansiAttribute, err := strconv.ParseInt(e, 10, 16)
				if err != nil {
					continue
				}

				if ansiAttribute == ANSI_SGR_RESET {
					attributes = aw.infoReset.Attributes
					continue
				}

				attributes = collectAnsiIntoWindowsAttributes(attributes, aw.infoReset.Attributes, SHORT(ansiAttribute))
			}
		}

		err = SetConsoleTextAttribute(aw.fd, attributes)
		if err != nil {
			return err
		}

	case "S", "T":
		// [incrementS -- Scroll window up
		// [incrementT -- Scroll window down

		windowHeight := info.Window.Bottom - info.Window.Top + 1
		increment := ac.paramAsSHORT(0, 1)
		if ac.Command == "S" {
			increment = -increment
		}

		rect := info.Window
		if ac.Command == "S" {
			if rect.Top > 0 {
				rect.Top = addInRange(rect.Top, increment, 0, info.Size.Y-windowHeight-1)
				rect.Bottom = rect.Top + windowHeight
			}
		} else if ac.Command == "T" {
			if rect.Bottom < info.Size.Y-1 {
				rect.Bottom = addInRange(rect.Bottom, increment, windowHeight, info.Size.Y-1)
				rect.Top = rect.Bottom - windowHeight
			}
		}

		err = SetConsoleWindowInfo(aw.fd, true, &rect)
		if err != nil {
			return err
		}

	case "]":
		// TODO(azlinux): Handle Linux console private CSI sequences
		/*
		   The following sequences are neither ECMA-48 nor native VT102.  They are
		   native  to the Linux console driver.  Colors are in SGR parameters: 0 =
		   black, 1 = red, 2 = green, 3 = brown, 4 = blue, 5 = magenta, 6 =  cyan,
		   7 = white.

		   ESC [ 1 ; n ]       Set color n as the underline color
		   ESC [ 2 ; n ]       Set color n as the dim color
		   ESC [ 8 ]           Make the current color pair the default attributes.
		   ESC [ 9 ; n ]       Set screen blank timeout to n minutes.
		   ESC [ 10 ; n ]      Set bell frequency in Hz.
		   ESC [ 11 ; n ]      Set bell duration in msec.
		   ESC [ 12 ; n ]      Bring specified console to the front.
		   ESC [ 13 ]          Unblank the screen.
		   ESC [ 14 ; n ]      Set the VESA powerdown interval in minutes.

		*/
	}

	return nil
}

// setCursorPosition sets the cursor to the specified position, bounded to the buffer size
func (aw *ansiWriter) setCursorPosition(position *COORD, sizeBuffer COORD) error {
	position.X = ensureInRange(position.X, 0, sizeBuffer.X-1)
	position.Y = ensureInRange(position.Y, 0, sizeBuffer.Y-1)
	return SetConsoleCursorPosition(aw.fd, position)
}

// setCursorVisible sets the cursor visbility
func (aw *ansiWriter) setCursorVisible(isVisible bool) (err error) {
	cursorInfo := CONSOLE_CURSOR_INFO{}

	err = GetConsoleCursorInfo(aw.fd, &cursorInfo)
	if err != nil {
		return err
	}

	cursorInfo.Visible = boolToBOOL(isVisible)
	err = SetConsoleCursorInfo(aw.fd, &cursorInfo)
	if err != nil {
		return err
	}

	return nil
}
