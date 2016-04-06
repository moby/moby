// +build windows

package dockerfile

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
)

// platformSupports is a short-term function to give users a quality error
// message if a Dockerfile uses a command not supported on the platform.
func platformSupports(command string) error {
	switch command {
	// TODO Windows TP5. Expose can be removed from here once TP4 is
	// no longer supported.
	case "expose":
		if !hcsshim.IsTP4() {
			break
		}
		fallthrough
	case "user", "stopsignal", "arg":
		return fmt.Errorf("The daemon on this platform does not support the command '%s'", command)
	}
	return nil
}
