//go:build !nydus
// +build !nydus

package cache

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/cache/config"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func needsForceCompression(ctx context.Context, cs content.Store, source ocispecs.Descriptor, refCfg config.RefConfig) bool {
	return refCfg.Compression.Force
}
