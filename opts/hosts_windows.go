// +build windows

package opts

// DefaultHost constant defines the default host string used by docker on Windows
var DefaultHost = "npipe://" + DefaultNamedPipe

// DefaultOrAltHost function returns the name of either the default host if found or the alternate Unix socket
func DefaultOrAltHost() string {
	return DefaultHost
}
