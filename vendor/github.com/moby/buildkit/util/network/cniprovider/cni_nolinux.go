//go:build !linux
// +build !linux

package cniprovider

import (
	"context"

	resourcestypes "github.com/moby/buildkit/executor/resources/types"
)

func (ns *cniNS) sample() (*resourcestypes.NetworkSample, error) {
	return nil, nil
}

func withDetachedNetNSIfAny(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
