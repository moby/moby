// +build !windows

package distribution

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/docker/distribution/xfer"
)

func (ld *v2LayerDescriptor) open(ctx context.Context) (distribution.ReadSeekCloser, error) {
	if len(ld.src.URLs) != 0 {
		return nil, xfer.DoNotRetry{Err: errNoForeignLayerSupport}
	}
	blobs := ld.repo.Blobs(ctx)
	return blobs.Open(ctx, ld.digest)
}
