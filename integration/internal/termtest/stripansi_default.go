package termtest // import "github.com/docker/docker/integration/internal/termtest"

import (
	"errors"
)

// StripANSICommands provides a dummy implementation for non-Windows platforms.
func StripANSICommands(input string) (string, error) {
	return input, errors.New("StripANSICommands is not implemented for this platform")
}
