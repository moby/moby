//go:build !nydus
// +build !nydus

package containerimage

import (
	"context"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func patchImageLayers(ctx context.Context, remote *solver.Remote, history []ocispecs.History, ref cache.ImmutableRef, opts *ImageCommitOpts, sg session.Group) (*solver.Remote, []ocispecs.History, error) {
	remote, history = normalizeLayersAndHistory(ctx, remote, history, ref, opts.OCITypes)
	return remote, history, nil
}
