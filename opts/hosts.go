package opts // import "github.com/docker/docker/opts"

import (
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/homedir"
	"github.com/pkg/errors"
)

const (
	// DefaultHTTPPort Default HTTP Port used if only the protocol is provided to -H flag e.g. dockerd -H tcp://
	// These are the IANA registered port numbers for use with Docker
	// see http://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.xhtml?search=docker
	DefaultHTTPPort = 2375 // Default HTTP Port
	// DefaultTLSHTTPPort Default HTTP Port used when TLS enabled
	DefaultTLSHTTPPort = 2376 // Default TLS encrypted HTTP Port
	// DefaultUnixSocket Path for the unix socket.
	// Docker daemon by default always listens on the default unix socket
	DefaultUnixSocket = "/var/run/docker.sock"
	// DefaultTCPHost constant defines the default host string used by docker on Windows
	DefaultTCPHost = "tcp://" + DefaultHTTPHost + ":2375"
	// DefaultTLSHost constant defines the default host string used by docker for TLS sockets
	DefaultTLSHost = "tcp://" + DefaultHTTPHost + ":2376"
	// DefaultNamedPipe defines the default named pipe used by docker on Windows
	DefaultNamedPipe = `//./pipe/docker_engine`
	// HostGatewayName is the string value that can be passed
	// to the IPAddr section in --add-host that is replaced by
	// the value of HostGatewayIP daemon config value
	HostGatewayName = "host-gateway"
)

// ValidateHost validates that the specified string is a valid host and returns it.
func ValidateHost(val string) (string, error) {
	host := strings.TrimSpace(val)
	// The empty string means default and is not handled by parseDaemonHost
	if host != "" {
		_, err := parseDaemonHost(host)
		if err != nil {
			return val, err
		}
	}
	// Note: unlike most flag validators, we don't return the mutated value here
	//       we need to know what the user entered later (using ParseHost) to adjust for TLS
	return val, nil
}

// ParseHost and set defaults for a Daemon host string.
// defaultToTLS is preferred over defaultToUnixXDG.
func ParseHost(defaultToTLS, defaultToUnixXDG bool, val string) (string, error) {
	host := strings.TrimSpace(val)
	if host == "" {
		if defaultToTLS {
			host = DefaultTLSHost
		} else if defaultToUnixXDG {
			runtimeDir, err := homedir.GetRuntimeDir()
			if err != nil {
				return "", err
			}
			socket := filepath.Join(runtimeDir, "docker.sock")
			host = "unix://" + socket
		} else {
			host = DefaultHost
		}
	} else {
		var err error
		host, err = parseDaemonHost(host)
		if err != nil {
			return val, err
		}
	}
	return host, nil
}

// parseDaemonHost parses the specified address and returns an address that will be used as the host.
// Depending on the address specified, this may return one of the global Default* strings defined in hosts.go.
func parseDaemonHost(addr string) (string, error) {
	addrParts := strings.SplitN(addr, "://", 2)
	if len(addrParts) == 1 && addrParts[0] != "" {
		addrParts = []string{"tcp", addrParts[0]}
	}

	switch addrParts[0] {
	case "tcp":
		return ParseTCPAddr(addr, DefaultTCPHost)
	case "unix":
		return parseSimpleProtoAddr("unix", addrParts[1], DefaultUnixSocket)
	case "npipe":
		return parseSimpleProtoAddr("npipe", addrParts[1], DefaultNamedPipe)
	case "fd":
		return addr, nil
	default:
		return "", errors.Errorf("invalid bind address (%s): unsupported proto '%s'", addr, addrParts[0])
	}
}

// parseSimpleProtoAddr parses and validates that the specified address is a valid
// socket address for simple protocols like unix and npipe. It returns a formatted
// socket address, either using the address parsed from addr, or the contents of
// defaultAddr if addr is a blank string.
func parseSimpleProtoAddr(proto, addr, defaultAddr string) (string, error) {
	addr = strings.TrimPrefix(addr, proto+"://")
	if strings.Contains(addr, "://") {
		return "", errors.Errorf("invalid proto, expected %s: %s", proto, addr)
	}
	if addr == "" {
		addr = defaultAddr
	}
	return proto + "://" + addr, nil
}

// ParseTCPAddr parses and validates that the specified address is a valid TCP
// address. It returns a formatted TCP address, either using the address parsed
// from tryAddr, or the contents of defaultAddr if tryAddr is a blank string.
// tryAddr is expected to have already been Trim()'d
// defaultAddr must be in the full `tcp://host:port` form
func ParseTCPAddr(tryAddr string, defaultAddr string) (string, error) {
	def, err := parseTCPAddr(defaultAddr, true)
	if err != nil {
		return "", errors.Wrapf(err, "invalid default address (%s)", defaultAddr)
	}

	addr, err := parseTCPAddr(tryAddr, false)
	if err != nil {
		return "", errors.Wrapf(err, "invalid bind address (%s)", tryAddr)
	}

	host := addr.Hostname()
	if host == "" {
		host = def.Hostname()
	}
	port := addr.Port()
	if port == "" {
		port = def.Port()
	}

	return "tcp://" + net.JoinHostPort(host, port), nil
}

// parseTCPAddr parses the given addr and validates if it is in the expected
// format. If strict is enabled, the address must contain a scheme (tcp://),
// a host (or IP-address) and a port number.
func parseTCPAddr(address string, strict bool) (*url.URL, error) {
	if !strict && !strings.Contains(address, "://") {
		address = "tcp://" + address
	}
	parsedURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme != "tcp" {
		return nil, errors.Errorf("unsupported proto '%s'", parsedURL.Scheme)
	}
	if parsedURL.Path != "" {
		return nil, errors.New("should not contain a path element")
	}
	if strict && parsedURL.Host == "" {
		return nil, errors.New("no host or IP address")
	}
	if parsedURL.Port() != "" || strict {
		if p, err := strconv.Atoi(parsedURL.Port()); err != nil || p == 0 {
			return nil, errors.Errorf("invalid port: %s", parsedURL.Port())
		}
	}
	return parsedURL, nil
}

// ValidateExtraHost validates that the specified string is a valid extrahost and returns it.
// ExtraHost is in the form of name:ip where the ip has to be a valid ip (IPv4 or IPv6).
func ValidateExtraHost(val string) (string, error) {
	// allow for IPv6 addresses in extra hosts by only splitting on first ":"
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || len(arr[0]) == 0 {
		return "", errors.Errorf("bad format for add-host: %q", val)
	}
	// Skip IPaddr validation for special "host-gateway" string
	if arr[1] != HostGatewayName {
		if _, err := ValidateIPAddress(arr[1]); err != nil {
			return "", errors.Errorf("invalid IP address in add-host: %q", arr[1])
		}
	}
	return val, nil
}
