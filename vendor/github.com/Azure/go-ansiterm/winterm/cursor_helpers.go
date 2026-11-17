//go:build windows

package winterm

const (
	horizontal = iota
	vertical
	line
	column
)

func (h *windowsAnsiEventHandler) getCursorWindow(info *CONSOLE_SCREEN_BUFFER_INFO) SMALL_RECT {
	if h.originMode {
		sr := h.effectiveSr(info.Window)
		return SMALL_RECT{
			Top:    sr.top,
			Bottom: sr.bottom,
			Left:   0,
			Right:  info.Size.X - 1,
		}
	} else {
		return SMALL_RECT{
			Top:    info.Window.Top,
			Bottom: info.Window.Bottom,
			Left:   0,
			Right:  info.Size.X - 1,
		}
	}
}

// setCursorPosition sets the cursor to the specified position, bounded to the screen size
func (h *windowsAnsiEventHandler) setCursorPosition(position COORD, window SMALL_RECT) error {
	position.X = ensureInRange(position.X, window.Left, window.Right)
	position.Y = ensureInRange(position.Y, window.Top, window.Bottom)
	err := SetConsoleCursorPosition(h.fd, position)
	if err != nil {
		return err
	}
	h.logf("Cursor position set: (%d, %d)", position.X, position.Y)
	return err
}

func (h *windowsAnsiEventHandler) moveCursorVertical(param int) error {
	return h.moveCursor(vertical, param)
}

func (h *windowsAnsiEventHandler) moveCursorHorizontal(param int) error {
	return h.moveCursor(horizontal, param)
}

func (h *windowsAnsiEventHandler) moveCursor(moveMode int, param int) error {
	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	position := info.CursorPosition
	switch moveMode {
	case horizontal:
		position.X += int16(param)
	case vertical:
		position.Y += int16(param)
	case line:
		position.X = 0
		position.Y += int16(param)
	case column:
		position.X = int16(param) - 1
	}

	return h.setCursorPosition(position, h.getCursorWindow(info))
}

func (h *windowsAnsiEventHandler) moveCursorLine(param int) error {
	return h.moveCursor(line, param)
}

func (h *windowsAnsiEventHandler) moveCursorColumn(param int) error {
	return h.moveCursor(column, param)
}
