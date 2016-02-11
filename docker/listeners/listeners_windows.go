package listeners

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

// Init creates new listeners for the server.
func Init(proto, addr, socketGroup string, tlsConfig *tls.Config) (ls []net.Listener, err error) {
	switch proto {
	case "tcp":
		l, err := initTCPSocket(addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)

	case "npipe":
		// allow Administrators and SYSTEM, plus whatever additional users or groups were specified
		sddl := "D:P(A;;GA;;;BA)(A;;GA;;;SY)"
		if socketGroup != "" {
			for _, g := range strings.Split(socketGroup, ",") {
				sid, err := winio.LookupSidByName(g)
				if err != nil {
					return nil, err
				}
				sddl += fmt.Sprintf("(A;;GRGW;;;%s)", sid)
			}
		}
		l, err := winio.ListenPipe(addr, sddl)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)

	default:
		return nil, errors.New("Invalid protocol format. Windows only supports tcp and npipe.")
	}

	return
}

// allocateDaemonPort ensures that there are no containers
// that try to use any port allocated for the docker server.
func allocateDaemonPort(addr string) error {
	return nil
}
