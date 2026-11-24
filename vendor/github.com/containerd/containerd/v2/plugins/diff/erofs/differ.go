/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package erofs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/diff"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/internal/erofsutils"
	"github.com/containerd/containerd/v2/pkg/tracing"

	"github.com/google/uuid"
)

var emptyDesc = ocispec.Descriptor{}

type differ interface {
	diff.Applier
	diff.Comparer
}

// erofsDiff does erofs comparison and application
type erofsDiff struct {
	store         content.Store
	mkfsExtraOpts []string
	// enableTarIndex enables generating tar index for tar content
	// instead of fully converting the tar to EROFS format
	enableTarIndex bool
	// enableDmverity enables formatting layers with dm-verity after creation
	enableDmverity bool
}

// DifferOpt is an option for configuring the erofs differ
type DifferOpt func(d *erofsDiff)

// WithMkfsOptions sets extra options for mkfs.erofs
func WithMkfsOptions(opts []string) DifferOpt {
	return func(d *erofsDiff) {
		d.mkfsExtraOpts = opts
	}
}

// WithTarIndexMode enables tar index mode for EROFS layers
func WithTarIndexMode() DifferOpt {
	return func(d *erofsDiff) {
		d.enableTarIndex = true
	}
}

// WithDmverity enables dm-verity formatting for EROFS layers
func WithDmverity() DifferOpt {
	return func(d *erofsDiff) {
		d.enableDmverity = true
	}
}

// NewErofsDiffer creates a new EROFS differ with the provided options
func NewErofsDiffer(store content.Store, opts ...DifferOpt) differ {
	d := &erofsDiff{
		store: store,
	}

	// Apply all options
	for _, opt := range opts {
		opt(d)
	}

	// Add default block size on darwin if not already specified
	d.mkfsExtraOpts = erofsutils.AddDefaultMkfsOpts(d.mkfsExtraOpts)

	return d
}

// spanPrefix names this package's tracing spans.
const spanPrefix = "plugins.diff.erofs"

func (s erofsDiff) Apply(ctx context.Context, desc ocispec.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (d ocispec.Descriptor, err error) {
	ctx, span := tracing.StartSpan(ctx, tracing.Name(spanPrefix, "Apply"))
	span.SetAttributes(
		tracing.Attribute("layer.media.type", desc.MediaType),
		tracing.Attribute("layer.media.size", desc.Size),
		tracing.Attribute("layer.media.digest", desc.Digest.String()),
	)

	t1 := time.Now()
	var mode string
	defer func() {
		span.SetStatus(err)
		span.End()

		if err == nil {
			log.G(ctx).WithFields(log.Fields{
				"d":      time.Since(t1),
				"digest": desc.Digest,
				"size":   desc.Size,
				"media":  desc.MediaType,
				"mode":   mode,
			}).Debugf("diff applied")
		}
	}()

	// Read-only mounts belong to a snapshot the snapshotter populated itself,
	// for example from a layer content cache. The layer is already there and is
	// shared with every other snapshot of it. Applying would write to content
	// this differ does not own. The unpacker reads the same mounts and skips
	// such a layer before it reaches here.
	//
	// The error is ErrFailedPrecondition. The diff service falls through to the
	// next differ only on ErrNotImplemented. Another differ would fail on this
	// layer for the same reason.
	if len(mounts) > 0 && mounts[len(mounts)-1].ReadOnly() {
		return emptyDesc, fmt.Errorf("cannot apply to a read-only snapshot, its content is already populated: %w", errdefs.ErrFailedPrecondition)
	}

	var (
		erofsLayerType string
		fastcopy       bool
	)
	diffLayerType := desc.MediaType
	native := erofsutils.IsErofsMediaType(diffLayerType)
	if native {
		base, ext, hasExt := strings.Cut(diffLayerType, "+")
		// Mimic the OCI layer for EROFS blobs for diff.NewProcessorChain(), so
		// there is no need to bother with too much unrelated logic for now.
		diffLayerType = ocispec.MediaTypeImageLayer
		if hasExt {
			// `+zstd` indicates that the original EROFS blob is additionally
			// compressed with standard zstd streams.
			// Only `+zstd` is considered since it is more performant than gzip
			// and has useful features like skippable frames.
			if ext != "zstd" {
				return emptyDesc, fmt.Errorf("unsupported erofs layer suffix: %s", ext)
			}
			diffLayerType = diffLayerType + "+zstd"
		} else {
			fastcopy = true
		}
		erofsLayerType = base
	} else if _, err := images.DiffCompression(ctx, diffLayerType); err != nil {
		return emptyDesc, fmt.Errorf("unsupported media type: %s", desc.MediaType)
	}

	switch {
	case fastcopy:
		mode = "fastcopy"
	case native:
		mode = "native"
	case s.enableTarIndex:
		mode = "tar-index"
	default:
		mode = "convert"
	}
	span.SetAttributes(tracing.Attribute("erofs.apply.mode", mode))

	var config diff.ApplyConfig
	for _, o := range opts {
		if err := o(ctx, desc, &config); err != nil {
			return emptyDesc, fmt.Errorf("failed to apply config opt: %w", err)
		}
	}

	layer, err := erofsutils.MountsToLayer(mounts)
	if err != nil {
		return emptyDesc, err
	}

	ra, err := s.store.ReaderAt(ctx, desc)
	if err != nil {
		return emptyDesc, fmt.Errorf("failed to get reader from content store: %w", err)
	}
	defer ra.Close()

	layerBlobPath := path.Join(layer, "layer.erofs")
	// Allow copy file range when there is an uncompressed native EROFS layer
	if fastcopy {
		f, err := os.Create(layerBlobPath)
		if err != nil {
			return emptyDesc, err
		}
		_, err = io.Copy(f, content.NewReader(ra))
		f.Close()
		if err != nil {
			return emptyDesc, err
		}
		log.G(ctx).WithField("path", layerBlobPath).Debug("Applied layer with uncompressed EROFS blob")
		return desc, nil
	}

	processor := diff.NewProcessorChain(diffLayerType, content.NewReader(ra))
	for {
		if processor, err = diff.GetProcessor(ctx, processor, config.ProcessorPayloads); err != nil {
			return emptyDesc, fmt.Errorf("failed to get stream processor for %s: %w", desc.MediaType, err)
		}
		if processor.MediaType() == ocispec.MediaTypeImageLayer {
			break
		}
	}
	defer processor.Close()

	digester := digest.Canonical.Digester()
	rc := &readCounter{
		r: io.TeeReader(processor, digester.Hash()),
	}

	// Choose between tar index or tar conversion mode
	// Generate deterministic UUID from layer digest
	u := uuid.NewSHA1(uuid.NameSpaceURL, []byte("erofs:blobs/"+desc.Digest))
	if native {
		f, err := os.Create(layerBlobPath)
		if err != nil {
			return emptyDesc, err
		}
		_, err = io.Copy(f, rc)
		f.Close()
		if err != nil {
			return emptyDesc, err
		}
		log.G(ctx).WithField("path", layerBlobPath).Debug("Applied layer with compressed EROFS blob")
	} else if s.enableTarIndex {
		// Use the tar index method: generate tar index and append tar
		err = erofsutils.GenerateTarIndexAndAppendTar(ctx, rc, layerBlobPath, u.String(), s.mkfsExtraOpts)
		if err != nil {
			return emptyDesc, fmt.Errorf("failed to generate tar index: %w", err)
		}
		log.G(ctx).WithField("path", layerBlobPath).Debug("Applied layer using tar index mode")
	} else {
		// Use the tar method: fully convert tar to EROFS
		err = erofsutils.ConvertTarErofs(ctx, rc, layerBlobPath, u.String(), s.mkfsExtraOpts)
		if err != nil {
			return emptyDesc, fmt.Errorf("failed to convert tar to erofs: %w", err)
		}
		log.G(ctx).WithField("path", layerBlobPath).Debug("Applied layer using tar conversion mode")
	}

	// Read any trailing data
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return emptyDesc, err
	}

	// Format with dm-verity if enabled
	if s.enableDmverity {
		if err := s.formatDmverityLayer(ctx, layerBlobPath); err != nil {
			return emptyDesc, fmt.Errorf("failed to format dm-verity layer: %w", err)
		}
	}

	if native {
		return ocispec.Descriptor{
			MediaType: erofsLayerType,
			Size:      rc.c,
			Digest:    digester.Digest(),
		}, nil
	}
	return ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Size:      rc.c,
		Digest:    digester.Digest(),
	}, nil
}

type readCounter struct {
	r io.Reader
	c int64
}

func (rc *readCounter) Read(p []byte) (n int, err error) {
	n, err = rc.r.Read(p)
	rc.c += int64(n)
	return
}
