// +build windows

package windows

// StdConsole is for when using a container non-interactively
type StdConsole struct {
}

// NewStdConsole returns a new StdConsole struct.
func NewStdConsole() *StdConsole {
	return &StdConsole{}
}

// Resize implements Resize method of Terminal interface.
func (s *StdConsole) Resize(h, w int) error {
	// we do not need to resize a non tty
	return nil
}

// Close implements Close method of Terminal interface.
func (s *StdConsole) Close() error {
	// nothing to close here
	return nil
}
