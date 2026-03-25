package container

import (
	"net"
	"os"
	"strings"

	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
)

func ParseExtraHosts(ips []*pb.HostIP) ([]executor.HostIP, error) {
	out := make([]executor.HostIP, len(ips))
	for i, hip := range ips {
		ip := net.ParseIP(hip.IP)
		if ip == nil {
			return nil, errors.Errorf("failed to parse IP %s", hip.IP)
		}
		out[i] = executor.HostIP{
			IP:   ip,
			Host: hip.Host,
		}
	}
	return out, nil
}

func isPathEscapesRootError(err error) bool {
	var pe *os.PathError
	if !errors.As(err, &pe) {
		return false
	}
	return strings.Contains(pe.Err.Error(), "path escapes")
}
