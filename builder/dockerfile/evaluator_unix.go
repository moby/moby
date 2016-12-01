// +build !windows

package dockerfile

import "fmt"

// platformSupports is a short-term function to give users a quality error
// message if a Dockerfile uses a command not supported on the platform.
func platformSupports(command string) error {
	switch command {
	case "getenv":
		return fmt.Errorf("The daemon on this platform does not support the command '%s'", command)
	}
	return nil
}
