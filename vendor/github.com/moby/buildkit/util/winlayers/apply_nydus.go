//go:build nydus
// +build nydus

package winlayers

import (
	"context"
	"io"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/mount"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	nydusify "github.com/containerd/nydus-snapshotter/pkg/converter"
)

func isNydusBlob(ctx context.Context, desc ocispecs.Descriptor) bool {
	if desc.Annotations == nil {
		return false
	}

	hasMediaType := desc.MediaType == nydusify.MediaTypeNydusBlob
	_, hasAnno := desc.Annotations[nydusify.LayerAnnotationNydusBlob]
	return hasMediaType && hasAnno
}

func (s *winApplier) apply(ctx context.Context, desc ocispecs.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (d ocispecs.Descriptor, err error) {
	if !isNydusBlob(ctx, desc) {
		return s.a.Apply(ctx, desc, mounts, opts...)
	}

	var ocidesc ocispecs.Descriptor
	if err := mount.WithTempMount(ctx, mounts, func(root string) error {
		ra, err := s.cs.ReaderAt(ctx, desc)
		if err != nil {
			return errors.Wrap(err, "get reader from content store")
		}
		defer ra.Close()

		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			if err := nydusify.Unpack(ctx, ra, pw, nydusify.UnpackOption{}); err != nil {
				pw.CloseWithError(errors.Wrap(err, "unpack nydus blob"))
			}
		}()
		defer pr.Close()

		digester := digest.Canonical.Digester()
		rc := &readCounter{
			r: io.TeeReader(pr, digester.Hash()),
		}

		if _, err := archive.Apply(ctx, root, rc); err != nil {
			return errors.Wrap(err, "apply nydus blob")
		}

		ocidesc = ocispecs.Descriptor{
			MediaType: ocispecs.MediaTypeImageLayer,
			Size:      rc.c,
			Digest:    digester.Digest(),
		}

		return nil
	}); err != nil {
		return ocispecs.Descriptor{}, err
	}

	return ocidesc, nil
}
