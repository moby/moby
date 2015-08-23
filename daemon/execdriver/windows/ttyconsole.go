// +build windows

package windows

import (
	"github.com/microsoft/hcsshim"
)

// TtyConsole implements the exec driver Terminal interface.
type TtyConsole struct {
	id        string
	processid uint32
}

// NewTtyConsole returns a new TtyConsole struct.
func NewTtyConsole(id string, processid uint32) *TtyConsole {
	tty := &TtyConsole{
		id:        id,
		processid: processid,
	}
	return tty
}

// Resize implements Resize method of Terminal interface.
func (t *TtyConsole) Resize(h, w int) error {
	return hcsshim.ResizeConsoleInComputeSystem(t.id, t.processid, h, w)
}

// Close implements Close method of Terminal interface.
func (t *TtyConsole) Close() error {
	return nil
}
