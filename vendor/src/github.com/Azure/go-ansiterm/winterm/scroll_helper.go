// +build windows

package winterm

func (h *WindowsAnsiEventHandler) scrollPageUp() error {
	return h.scrollPage(1)
}

func (h *WindowsAnsiEventHandler) scrollPageDown() error {
	return h.scrollPage(-1)
}

func (h *WindowsAnsiEventHandler) scrollPage(param int) error {
	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	tmpScrollTop := h.sr.top
	tmpScrollBottom := h.sr.bottom

	// Set scroll region to whole window
	h.sr.top = 0
	h.sr.bottom = int(info.Size.Y - 1)

	err = h.scroll(param)

	h.sr.top = tmpScrollTop
	h.sr.bottom = tmpScrollBottom

	return err
}

func (h *WindowsAnsiEventHandler) scrollUp(param int) error {
	return h.scroll(param)
}

func (h *WindowsAnsiEventHandler) scrollDown(param int) error {
	return h.scroll(-param)
}

func (h *WindowsAnsiEventHandler) scroll(param int) error {

	info, err := GetConsoleScreenBufferInfo(h.fd)
	if err != nil {
		return err
	}

	logger.Infof("scroll: scrollTop: %d, scrollBottom: %d", h.sr.top, h.sr.bottom)
	logger.Infof("scroll: windowTop: %d, windowBottom: %d", info.Window.Top, info.Window.Bottom)

	rect := info.Window

	// Current scroll region in Windows backing buffer coordinates
	top := rect.Top + SHORT(h.sr.top)
	bottom := rect.Top + SHORT(h.sr.bottom)

	// Area from backing buffer to be copied
	scrollRect := SMALL_RECT{
		Top:    top + SHORT(param),
		Bottom: bottom + SHORT(param),
		Left:   rect.Left,
		Right:  rect.Right,
	}

	// Clipping region should be the original scroll region
	clipRegion := SMALL_RECT{
		Top:    top,
		Bottom: bottom,
		Left:   rect.Left,
		Right:  rect.Right,
	}

	// Origin to which area should be copied
	destOrigin := COORD{
		X: rect.Left,
		Y: top,
	}

	char := CHAR_INFO{
		UnicodeChar: ' ',
		Attributes:  0,
	}

	if err := ScrollConsoleScreenBuffer(h.fd, scrollRect, clipRegion, destOrigin, char); err != nil {
		return err
	}

	return nil
}
