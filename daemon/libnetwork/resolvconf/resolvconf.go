// Package resolvconf provides utility code to get the host's "resolv.conf" path.
package resolvconf

import "github.com/moby/moby/v2/daemon/libnetwork/internal/resolvconf"

// Path is an alias for [resolvconf.Path], which is internal to libnetwork.
//
// FIXME(thaJeztah): remove this when possible. This is only used in [github.com/moby/moby/v2/daemon.setupResolvConf].
// Either we can move "libnetwork/internal/resolvconf" to "daemon/internal",
// or move the "Path" function to daemon/config (considering it a default
// for the daemon's config).
func Path() string {
	return resolvconf.Path()
}
