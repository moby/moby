//go:build !nydus
// +build !nydus

package winlayers

import (
	"context"

	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/mount"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func (s *winApplier) apply(ctx context.Context, desc ocispecs.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (d ocispecs.Descriptor, err error) {
	return s.a.Apply(ctx, desc, mounts, opts...)
}
