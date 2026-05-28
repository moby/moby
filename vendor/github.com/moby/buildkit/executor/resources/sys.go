package resources

import resourcestypes "github.com/moby/buildkit/executor/resources/types"

type SysSampler = Sub[*resourcestypes.SysSample]

func NewSysSampler() (*Sampler[*resourcestypes.SysSample], error) {
	return newSysSampler()
}
