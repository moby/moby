//go:build !linux
// +build !linux

package cache

import (
	"context"

	"github.com/moby/buildkit/util/compression"
	"github.com/containerd/containerd/mount"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func (sr *immutableRef) tryComputeOverlayBlob(ctx context.Context, lower, upper []mount.Mount, mediaType string, ref string, compressorFunc compression.Compressor) (_ ocispecs.Descriptor, ok bool, err error) {
	return ocispecs.Descriptor{}, true, errors.Errorf("overlayfs-based diff computing is unsupported")
}
