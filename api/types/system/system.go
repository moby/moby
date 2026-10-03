package system // import "github.com/docker/docker/api/types/system"

// ListenerInfo provides information about an address that the API is listening on.
type ListenerInfo struct {
	// Address is the address that the daemon API is listening on.
	Address string
	// Insecure indicates if the address is insecure, which could be either
	// if the daemon is configured without TLS client verification, or if
	// TLS verify is disabled.
	Insecure bool
}
