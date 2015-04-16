// +build windows

package winterm

const (
	Horizontal = iota
	Vertical
)

// setCursorPosition sets the cursor to the specified position, bounded to the buffer size
func (h *WindowsAnsiEventHandler) setCursorPosition(position COORD, sizeBuffer COORD) error {
	position.X = ensureInRange(position.X, 0, sizeBuffer.X-1)
	position.Y = ensureInRange(position.Y, 0, sizeBuffer.Y-1)
	return SetConsoleCursorPosition(h.fd, position)
}

func (h *WindowsAnsiEventHandler) moveCursorVertical(param int) error {
	return h.moveCursor(Vertical, param)
}

func (h *WindowsAnsiEventHandler) moveCursorHorizontal(param int) error {
	return h.moveCursor(Horizontal, param)
}

func (h *WindowsAnsiEventHandler) moveCursor(moveMode int, param int) error {
	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	position := info.CursorPosition
	switch moveMode {
	case Horizontal:
		position.X = AddInRange(position.X, SHORT(param), info.Window.Left, info.Window.Right)
	case Vertical:
		position.Y = AddInRange(position.Y, SHORT(param), info.Window.Top, info.Window.Bottom)
	}

	if err = h.setCursorPosition(position, info.Size); err != nil {
		return err
	}

	logger.Infof("Cursor position set: (%d, %d)", position.X, position.Y)

	return nil
}

func (h *WindowsAnsiEventHandler) moveCursorLine(param int) error {
	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	position := info.CursorPosition
	position.X = 0
	position.Y = AddInRange(position.Y, SHORT(param), info.Window.Top, info.Window.Bottom)

	if err = h.setCursorPosition(position, info.Size); err != nil {
		return err
	}

	return nil
}

func (h *WindowsAnsiEventHandler) moveCursorColumn(param int) error {
	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	position := info.CursorPosition
	position.X = AddInRange(SHORT(param), -1, info.Window.Left, info.Window.Right)

	if err = h.setCursorPosition(position, info.Size); err != nil {
		return err
	}

	return nil
}
