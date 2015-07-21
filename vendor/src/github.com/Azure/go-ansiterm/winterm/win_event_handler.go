// +build windows

package winterm

import (
	"io/ioutil"
	"os"
	"strconv"

	. "github.com/Azure/go-ansiterm"
	"github.com/Sirupsen/logrus"
)

var logger *logrus.Logger

type WindowsAnsiEventHandler struct {
	fd        uintptr
	file      *os.File
	infoReset *CONSOLE_SCREEN_BUFFER_INFO
	sr        scrollRegion
}

func CreateWinEventHandler(fd uintptr, file *os.File) *WindowsAnsiEventHandler {
	logFile := ioutil.Discard

	if isDebugEnv := os.Getenv(LogEnv); isDebugEnv == "1" {
		logFile, _ = os.Create("winEventHandler.log")
	}

	logger = &logrus.Logger{
		Out:       logFile,
		Formatter: new(logrus.TextFormatter),
		Level:     logrus.DebugLevel,
	}

	infoReset, err := GetConsoleScreenBufferInfo(fd)
	if err != nil {
		return nil
	}

	sr := scrollRegion{int(infoReset.Window.Top), int(infoReset.Window.Bottom)}

	return &WindowsAnsiEventHandler{
		fd:        fd,
		file:      file,
		infoReset: infoReset,
		sr:        sr,
	}
}

type scrollRegion struct {
	top    int
	bottom int
}

func (h *WindowsAnsiEventHandler) Print(b byte) error {
	logger.Infof("Print: [%v]", string(b))

	bytes := []byte{b}

	_, err := h.file.Write(bytes)
	if err != nil {
		return err
	}

	return nil
}

func (h *WindowsAnsiEventHandler) Execute(b byte) error {
	logger.Infof("Execute %#x", b)

	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	if int(info.CursorPosition.Y) == h.sr.bottom {
		if ANSI_LINE_FEED == b {
			// Scroll up one row if we attempt to line feed at the bottom
			// of the scroll region
			if err := h.scrollUp(1); err != nil {
				return err
			}

			// Clear line
			// if err := h.CUD(1); err != nil {
			// 	return err
			// }
			if err := h.EL(0); err != nil {
				return err
			}
		}
	}

	if ANSI_BEL <= b && b <= ANSI_CARRIAGE_RETURN {
		return h.Print(b)
	}

	return nil
}

func (h *WindowsAnsiEventHandler) CUU(param int) error {
	logger.Infof("CUU: [%v]", []string{strconv.Itoa(param)})
	return h.moveCursorVertical(-param)
}

func (h *WindowsAnsiEventHandler) CUD(param int) error {
	logger.Infof("CUD: [%v]", []string{strconv.Itoa(param)})
	return h.moveCursorVertical(param)
}

func (h *WindowsAnsiEventHandler) CUF(param int) error {
	logger.Infof("CUF: [%v]", []string{strconv.Itoa(param)})
	return h.moveCursorHorizontal(param)
}

func (h *WindowsAnsiEventHandler) CUB(param int) error {
	logger.Infof("CUB: [%v]", []string{strconv.Itoa(param)})
	return h.moveCursorHorizontal(-param)
}

func (h *WindowsAnsiEventHandler) CNL(param int) error {
	logger.Infof("CNL: [%v]", []string{strconv.Itoa(param)})
	return h.moveCursorLine(param)
}

func (h *WindowsAnsiEventHandler) CPL(param int) error {
	logger.Infof("CPL: [%v]", []string{strconv.Itoa(param)})
	return h.moveCursorLine(-param)
}

func (h *WindowsAnsiEventHandler) CHA(param int) error {
	logger.Infof("CHA: [%v]", []string{strconv.Itoa(param)})
	return h.moveCursorColumn(param)
}

func (h *WindowsAnsiEventHandler) CUP(row int, col int) error {
	rowStr, colStr := strconv.Itoa(row), strconv.Itoa(col)
	logger.Infof("CUP: [%v]", []string{rowStr, colStr})
	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	rect := info.Window
	rowS := AddInRange(SHORT(row-1), rect.Top, rect.Top, rect.Bottom)
	colS := AddInRange(SHORT(col-1), rect.Left, rect.Left, rect.Right)
	position := COORD{colS, rowS}

	return h.setCursorPosition(position, info.Size)
}

func (h *WindowsAnsiEventHandler) HVP(row int, col int) error {
	rowS, colS := strconv.Itoa(row), strconv.Itoa(row)
	logger.Infof("HVP: [%v]", []string{rowS, colS})
	return h.CUP(row, col)
}

func (h *WindowsAnsiEventHandler) DECTCEM(visible bool) error {
	logger.Infof("DECTCEM: [%v]", []string{strconv.FormatBool(visible)})

	return nil
}

func (h *WindowsAnsiEventHandler) ED(param int) error {
	logger.Infof("ED: [%v]", []string{strconv.Itoa(param)})

	// [J  -- Erases from the cursor to the end of the screen, including the cursor position.
	// [1J -- Erases from the beginning of the screen to the cursor, including the cursor position.
	// [2J -- Erases the complete display. The cursor does not move.
	// [3J -- Erases the complete display and backing buffer, cursor moves to (0,0)
	// Notes:
	// -- ANSI.SYS always moved the cursor to (0,0) for both [2J and [3J
	// -- Clearing the entire buffer, versus just the Window, works best for Windows Consoles

	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	var start COORD
	var end COORD

	switch param {
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

	err = h.clearRange(info.Attributes, start, end)
	if err != nil {
		return err
	}

	if param == 2 || param == 3 {
		err = h.setCursorPosition(COORD{0, 0}, info.Size)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *WindowsAnsiEventHandler) EL(param int) error {
	logger.Infof("EL: [%v]", strconv.Itoa(param))

	// [K  -- Erases from the cursor to the end of the line, including the cursor position.
	// [1K -- Erases from the beginning of the line to the cursor, including the cursor position.
	// [2K -- Erases the complete line.

	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	var start COORD
	var end COORD

	switch param {
	case 0:
		start = info.CursorPosition
		end = COORD{info.Size.X, info.CursorPosition.Y}

	case 1:
		start = COORD{0, info.CursorPosition.Y}
		end = info.CursorPosition

	case 2:
		start = COORD{0, info.CursorPosition.Y}
		end = COORD{info.Size.X, info.CursorPosition.Y}
	}

	err = h.clearRange(info.Attributes, start, end)
	if err != nil {
		return err
	}

	return nil
}

func (h *WindowsAnsiEventHandler) IL(param int) error {
	logger.Infof("IL: [%v]", strconv.Itoa(param))
	if err := h.scrollDown(param); err != nil {
		return err
	}

	return h.EL(2)
}

func (h *WindowsAnsiEventHandler) DL(param int) error {
	logger.Infof("DL: [%v]", strconv.Itoa(param))
	return h.scrollUp(param)
}

func (h *WindowsAnsiEventHandler) SGR(params []int) error {
	strings := []string{}
	for _, v := range params {
		logger.Infof("SGR: [%v]", strings)
		strings = append(strings, strconv.Itoa(v))
	}

	logger.Infof("SGR: [%v]", strings)

	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	attributes := info.Attributes
	if len(params) <= 0 {
		attributes = h.infoReset.Attributes
	} else {
		for _, attr := range params {

			if attr == ANSI_SGR_RESET {
				attributes = h.infoReset.Attributes
				continue
			}

			attributes = collectAnsiIntoWindowsAttributes(attributes, h.infoReset.Attributes, SHORT(attr))
		}
	}

	err = SetConsoleTextAttribute(h.fd, attributes)
	if err != nil {
		return err
	}

	return nil
}

func (h *WindowsAnsiEventHandler) SU(param int) error {
	logger.Infof("SU: [%v]", []string{strconv.Itoa(param)})
	return h.scrollPageUp()
}

func (h *WindowsAnsiEventHandler) SD(param int) error {
	logger.Infof("SD: [%v]", []string{strconv.Itoa(param)})
	return h.scrollPageDown()
}

func (h *WindowsAnsiEventHandler) DA(params []string) error {
	logger.Infof("DA: [%v]", params)

	// See the site below for details of the device attributes command
	// http://vt100.net/docs/vt220-rm/chapter4.html

	// First character of first parameter string is '>'
	if params[0][0] == '>' {
		// Secondary device attribute request:
		// Respond with:
		// "I am a VT220 version 1.0, no options.
		//                    CSI     >     1     ;     1     0     ;     0     c    CR    LF
		bytes := []byte{CSI_ENTRY, 0x3E, 0x31, 0x3B, 0x31, 0x30, 0x3B, 0x30, 0x63, 0x0D, 0x0A}

		for _, b := range bytes {
			h.Print(b)
		}
	} else {
		// Primary device attribute request:
		// Respond with:
		// "I am a service class 2 terminal (62) with 132 columns (1),
		// printer port (2), selective erase (6), DRCS (7), UDK (8),
		// and I support 7-bit national replacement character sets (9)."
		//                    CSI     ?     6     2     ;     1     ;     2     ;     6     ;     7     ;     8     ;     9     c    CR    LF
		bytes := []byte{CSI_ENTRY, 0x3F, 0x36, 0x32, 0x3B, 0x31, 0x3B, 0x32, 0x3B, 0x36, 0x3B, 0x37, 0x3B, 0x38, 0x3B, 0x39, 0x63, 0x0D, 0x0A}

		for _, b := range bytes {
			h.Print(b)
		}
	}

	return nil
}

func (h *WindowsAnsiEventHandler) DECSTBM(top int, bottom int) error {
	logger.Infof("DECSTBM: [%d, %d]", top, bottom)

	// Windows is 0 indexed, Linux is 1 indexed
	h.sr.top = top - 1
	h.sr.bottom = bottom - 1

	return nil
}

func (h *WindowsAnsiEventHandler) RI() error {
	logger.Info("RI: []")

	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	if info.Window.Top == info.CursorPosition.Y {
		if err := h.scrollPageDown(); err != nil {
			return err
		}

		return h.EL(2)
	} else {
		return h.CUU(1)
	}
}
