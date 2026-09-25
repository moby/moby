//go:build !nydus

package winlayers

import (
	"context"

	"github.com/containerd/containerd/v2/core/diff"
	"github.com/containerd/containerd/v2/core/mount"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func (s *winApplier) apply(ctx context.Context, desc ocispecs.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (d ocispecs.Descriptor, err error) {
	return s.a.Apply(ctx, desc, mounts, opts...)
}
