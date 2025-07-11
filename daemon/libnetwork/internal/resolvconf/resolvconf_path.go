package resolvconf

import (
	"context"
	"net/netip"
	"sync"

	"github.com/containerd/log"
)

const (
	// defaultPath is the default path to the resolv.conf that contains information to resolve DNS. See Path().
	defaultPath = "/etc/resolv.conf"
	// alternatePath is a path different from defaultPath, that may be used to resolve DNS. See Path().
	alternatePath = "/run/systemd/resolve/resolv.conf"
)

// For Path to detect systemd (only needed for legacy networking).
var (
	detectSystemdResolvConfOnce sync.Once
	pathAfterSystemdDetection   = defaultPath
)

// Path returns the path to the resolv.conf file that libnetwork should use.
//
// When /etc/resolv.conf contains 127.0.0.53 as the only nameserver, then
// it is assumed systemd-resolved manages DNS. Because inside the container 127.0.0.53
// is not a valid DNS server, Path() returns /run/systemd/resolve/resolv.conf
// which is the resolv.conf that systemd-resolved generates and manages.
// Otherwise Path() returns /etc/resolv.conf.
//
// Errors are silenced as they will inevitably resurface at future open/read calls.
//
// More information at https://www.freedesktop.org/software/systemd/man/systemd-resolved.service.html#/etc/resolv.conf
//
// TODO(robmry) - alternatePath is only needed for legacy networking ...
//
//	Host networking can use the host's resolv.conf as-is, and with an internal
//	resolver it's also possible to use nameservers on the host's loopback
//	interface. Once legacy networking is removed, this can always return
//	defaultPath.
func Path() string {
	detectSystemdResolvConfOnce.Do(func() {
		rc, err := Load(defaultPath)
		if err != nil {
			// silencing error as it will resurface at next calls trying to read defaultPath
			return
		}
		ns := rc.nameServers
		if len(ns) == 1 && ns[0] == netip.MustParseAddr("127.0.0.53") {
			pathAfterSystemdDetection = alternatePath
			log.G(context.TODO()).Infof("detected 127.0.0.53 nameserver, assuming systemd-resolved, so using resolv.conf: %s", alternatePath)
		}
	})
	return pathAfterSystemdDetection
}
