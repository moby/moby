package gitutil

import (
	"strings"

	"github.com/moby/buildkit/util/sshutil"
)

const (
	HTTPProtocol = iota + 1
	HTTPSProtocol
	SSHProtocol
	GitProtocol
	UnknownProtocol
)

// ParseProtocol parses a git URL and returns the remote url and protocol type
func ParseProtocol(remote string) (string, int) {
	prefixes := map[string]int{
		"http://":  HTTPProtocol,
		"https://": HTTPSProtocol,
		"git://":   GitProtocol,
		"ssh://":   SSHProtocol,
	}
	protocolType := UnknownProtocol
	for prefix, potentialType := range prefixes {
		if strings.HasPrefix(remote, prefix) {
			remote = strings.TrimPrefix(remote, prefix)
			protocolType = potentialType
		}
	}

	if protocolType == UnknownProtocol && sshutil.IsImplicitSSHTransport(remote) {
		protocolType = SSHProtocol
	}

	// remove name from ssh
	if protocolType == SSHProtocol {
		parts := strings.SplitN(remote, "@", 2)
		if len(parts) == 2 {
			remote = parts[1]
		}
	}

	return remote, protocolType
}
