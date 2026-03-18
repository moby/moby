//go:build !windows

package netproviders

import (
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/network"
)

func getHostProvider() (network.Provider, bool) {
	return network.NewHostProvider(), true
}

func getFallback() (network.Provider, string) {
	bklog.L.Warn("using host network as the default")
	return network.NewHostProvider(), "host"
}
