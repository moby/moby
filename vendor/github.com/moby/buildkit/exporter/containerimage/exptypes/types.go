package exptypes

import (
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

const ExporterImageConfigKey = "containerimage.config"
const ExporterInlineCache = "containerimage.inlinecache"
const ExporterPlatformsKey = "refs.platforms"

const EmptyGZLayer = digest.Digest("sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1")

type Platforms struct {
	Platforms []Platform
}

type Platform struct {
	ID       string
	Platform specs.Platform
}
