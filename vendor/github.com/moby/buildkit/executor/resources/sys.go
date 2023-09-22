package resources

import "github.com/moby/buildkit/executor/resources/types"

type SysSampler = Sub[*types.SysSample]

func NewSysSampler() (*Sampler[*types.SysSample], error) {
	return newSysSampler()
}
