//go:build linux
// +build linux

package cache

import (
	"bufio"
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/overlay"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var emptyDesc = ocispecs.Descriptor{}

// computeOverlayBlob provides overlayfs-specialized method to compute
// diff between lower and upper snapshot. If the passed mounts cannot
// be computed (e.g. because the mounts aren't overlayfs), it returns
// an error.
func (sr *immutableRef) tryComputeOverlayBlob(ctx context.Context, lower, upper []mount.Mount, mediaType string, ref string, compressorFunc compression.Compressor) (_ ocispecs.Descriptor, ok bool, err error) {
	// Get upperdir location if mounts are overlayfs that can be processed by this differ.
	upperdir, err := overlay.GetUpperdir(lower, upper)
	if err != nil {
		// This is not an overlayfs snapshot. This is not an error so don't return error here
		// and let the caller fallback to another differ.
		return emptyDesc, false, nil
	}

	cw, err := sr.cm.ContentStore.Writer(ctx,
		content.WithRef(ref),
		content.WithDescriptor(ocispecs.Descriptor{
			MediaType: mediaType, // most contentstore implementations just ignore this
		}))
	if err != nil {
		return emptyDesc, false, errors.Wrap(err, "failed to open writer")
	}
	defer func() {
		if cw != nil {
			if cerr := cw.Close(); cerr != nil {
				bklog.G(ctx).WithError(cerr).Warnf("failed to close writer %q", ref)
			}
		}
	}()

	bufW := bufio.NewWriterSize(cw, 128*1024)
	var labels map[string]string
	if compressorFunc != nil {
		dgstr := digest.SHA256.Digester()
		compressed, err := compressorFunc(bufW, mediaType)
		if err != nil {
			return emptyDesc, false, errors.Wrap(err, "failed to get compressed stream")
		}
		// Close ensure compressorFunc does some finalization works.
		defer compressed.Close()
		if err := overlay.WriteUpperdir(ctx, io.MultiWriter(compressed, dgstr.Hash()), upperdir, lower); err != nil {
			return emptyDesc, false, errors.Wrap(err, "failed to write compressed diff")
		}
		if err := compressed.Close(); err != nil {
			return emptyDesc, false, errors.Wrap(err, "failed to close compressed diff writer")
		}
		if labels == nil {
			labels = map[string]string{}
		}
		labels[containerdUncompressed] = dgstr.Digest().String()
	} else {
		if err = overlay.WriteUpperdir(ctx, bufW, upperdir, lower); err != nil {
			return emptyDesc, false, errors.Wrap(err, "failed to write diff")
		}
	}

	if err := bufW.Flush(); err != nil {
		return emptyDesc, false, errors.Wrap(err, "failed to flush diff")
	}
	var commitopts []content.Opt
	if labels != nil {
		commitopts = append(commitopts, content.WithLabels(labels))
	}
	dgst := cw.Digest()
	if err := cw.Commit(ctx, 0, dgst, commitopts...); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return emptyDesc, false, errors.Wrap(err, "failed to commit")
		}
	}
	if err := cw.Close(); err != nil {
		return emptyDesc, false, err
	}
	cw = nil
	cinfo, err := sr.cm.ContentStore.Info(ctx, dgst)
	if err != nil {
		return emptyDesc, false, errors.Wrap(err, "failed to get info from content store")
	}
	if cinfo.Labels == nil {
		cinfo.Labels = make(map[string]string)
	}
	// Set uncompressed label if digest already existed without label
	if _, ok := cinfo.Labels[containerdUncompressed]; !ok {
		cinfo.Labels[containerdUncompressed] = labels[containerdUncompressed]
		if _, err := sr.cm.ContentStore.Update(ctx, cinfo, "labels."+containerdUncompressed); err != nil {
			return emptyDesc, false, errors.Wrap(err, "error setting uncompressed label")
		}
	}

	return ocispecs.Descriptor{
		MediaType: mediaType,
		Size:      cinfo.Size,
		Digest:    cinfo.Digest,
	}, true, nil
}
