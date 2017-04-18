// +build !windows

package opts

import (
	"fmt"
	"os"
)

// DefaultHost constant defines the default host string used by docker on other hosts than Windows
var DefaultHost = fmt.Sprintf("unix://%s", DefaultUnixSocket)

// DefaultOrAltHost function returns the name of either the default host if found or the alternate Unix socket
func DefaultOrAltHost() string {
	fi, err := os.Stat(DefaultUnixSocket)
	if err == nil && fi.Mode()&os.ModeSocket != 0 {
		return DefaultHost
	}
	fi, err = os.Stat(AltUnixSocket)
	if err == nil && fi.Mode()&os.ModeSocket != 0 {
		return fmt.Sprintf("unix://%s", AltUnixSocket)
	}
	// if none found do not change historic behaviour, for error messages
	return DefaultHost
}
