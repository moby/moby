package listeners

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/docker/go-connections/sockets"
)

// Init creates new listeners for the server.
func Init(proto, addr, socketGroup string, tlsConfig *tls.Config) ([]net.Listener, error) {
	ls := []net.Listener{}

	switch proto {
	case "tcp":
		l, err := sockets.NewTCPSocket(addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)

	case "npipe":
		// Windows allows a comma-separated list of groups and/or users to be set.
		var additionalUsersAndGroups []string
		if socketGroup != "" {
			additionalUsersAndGroups = strings.Split(socketGroup, ",")
		}
		sddl, err := getSecurityDescriptor(additionalUsersAndGroups)
		if err != nil {
			return nil, err
		}
		l, err := winio.ListenPipe(addr, &winio.PipeConfig{
			SecurityDescriptor: sddl,
			MessageMode:        true,  // Use message mode so that CloseWrite() is supported
			InputBufferSize:    65536, // Use 64KB buffers to improve performance
			OutputBufferSize:   65536,
		})
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)

	default:
		return nil, fmt.Errorf("invalid protocol format: windows only supports tcp and npipe")
	}

	return ls, nil
}

// Default DACL allows Administrators and LocalSystem full access;
//
// - D:P: DACL without inheritance (protected, (P)).
// - (A;;GA;;;BA): Allow full access (GA) for built-in Administrators (BA).
// - (A;;GA;;;SY); Allow full access (GA) for LocalSystem (SY).
// - Any other user is denied access.
const defaultPermissions = "D:P(A;;GA;;;BA)(A;;GA;;;SY)"

// getSecurityDescriptor returns the DACL for the API socket or named pipe.
//
// By default, it grants [defaultPermissions], but allows for additional
// users and groups to get generic read (GR) and write (GW) access. It
// returns an error when failing to resolve any of the additional users
// and groups.
func getSecurityDescriptor(additionalUsersAndGroups []string) (sddl string, _ error) {
	sddl = defaultPermissions

	// Grant generic read (GR) and write (GW) access to whatever
	// additional users or groups were specified.
	for _, g := range additionalUsersAndGroups {
		sid, err := winio.LookupSidByName(strings.TrimSpace(g))
		if err != nil {
			return "", err
		}
		sddl += fmt.Sprintf("(A;;GRGW;;;%s)", sid)
	}
	return sddl, nil
}
