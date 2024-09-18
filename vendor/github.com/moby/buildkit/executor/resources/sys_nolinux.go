//go:build !linux

package resources

import resourcestypes "github.com/moby/buildkit/executor/resources/types"

func newSysSampler() (*Sampler[*resourcestypes.SysSample], error) {
	return nil, nil
}
