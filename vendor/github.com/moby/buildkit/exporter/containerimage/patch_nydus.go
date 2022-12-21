//go:build nydus
// +build nydus

package containerimage

import (
	"context"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// patchImageLayers appends an extra nydus bootstrap layer
// to the manifest of nydus image, normalizes layers and
// history. The nydus bootstrap layer represents the whole
// metadata of filesystem view for the entire image.
func patchImageLayers(ctx context.Context, remote *solver.Remote, history []ocispecs.History, ref cache.ImmutableRef, opts *ImageCommitOpts, sg session.Group) (*solver.Remote, []ocispecs.History, error) {
	if opts.RefCfg.Compression.Type != compression.Nydus {
		remote, history = normalizeLayersAndHistory(ctx, remote, history, ref, opts.OCITypes)
		return remote, history, nil
	}

	desc, err := cache.MergeNydus(ctx, ref, opts.RefCfg.Compression, sg)
	if err != nil {
		return nil, nil, errors.Wrap(err, "merge nydus layer")
	}
	remote.Descriptors = append(remote.Descriptors, *desc)

	remote, history = normalizeLayersAndHistory(ctx, remote, history, ref, opts.OCITypes)
	return remote, history, nil
}
