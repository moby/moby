//go:build !linux
// +build !linux

package cniprovider

import (
	"github.com/moby/buildkit/util/network"
)

func (ns *cniNS) sample() (*network.Sample, error) {
	return nil, nil
}
