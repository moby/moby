//go:build !linux
// +build !linux

package cniprovider

import (
	"context"

	"github.com/moby/buildkit/util/network"
)

func (ns *cniNS) sample() (*network.Sample, error) {
	return nil, nil
}

func withDetachedNetNSIfAny(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
