//go:build !linux
// +build !linux

package cache

import (
	"context"

	"github.com/containerd/containerd/mount"
	"github.com/moby/buildkit/util/compression"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func (sr *immutableRef) tryComputeOverlayBlob(_ context.Context, _, _ []mount.Mount, _ string, _ string, _ compression.Compressor) (_ ocispecs.Descriptor, ok bool, err error) {
	return ocispecs.Descriptor{}, true, errors.Errorf("overlayfs-based diff computing is unsupported")
}
