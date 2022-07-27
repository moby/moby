//go:build windows
// +build windows

package netproviders

import (
	"github.com/moby/buildkit/util/network"
	"github.com/sirupsen/logrus"
)

func getHostProvider() (network.Provider, bool) {
	return nil, false
}

func getFallback() (network.Provider, string) {
	logrus.Warn("using null network as the default")
	return network.NewNoneProvider(), ""
}
