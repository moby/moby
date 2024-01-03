//go:build !linux

package resources

import "github.com/moby/buildkit/executor/resources/types"

func newSysSampler() (*Sampler[*types.SysSample], error) {
	return nil, nil
}
