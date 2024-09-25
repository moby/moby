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

package containerd

import (
	"context"
	"errors"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/semaphore"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/pkg/unpack"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/schema1" //nolint:staticcheck // Ignore SA1019. Need to keep deprecated package for compatibility.
	"github.com/containerd/containerd/tracing"
	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
)

const (
	pullSpanPrefix = "pull"
)

// Pull downloads the provided content into containerd's content store
// and returns a platform specific image object
func (c *Client) Pull(ctx context.Context, ref string, opts ...RemoteOpt) (_ Image, retErr error) {
	ctx, span := tracing.StartSpan(ctx, tracing.Name(pullSpanPrefix, "Pull"))
	defer span.End()

	pullCtx := defaultRemoteContext()

	for _, o := range opts {
		if err := o(c, pullCtx); err != nil {
			return nil, err
		}
	}

	if pullCtx.PlatformMatcher == nil {
		if len(pullCtx.Platforms) > 1 {
			return nil, errors.New("cannot pull multiplatform image locally, try Fetch")
		} else if len(pullCtx.Platforms) == 0 {
			pullCtx.PlatformMatcher = c.platform
		} else {
			p, err := platforms.Parse(pullCtx.Platforms[0])
			if err != nil {
				return nil, fmt.Errorf("invalid platform %s: %w", pullCtx.Platforms[0], err)
			}

			pullCtx.PlatformMatcher = platforms.Only(p)
		}
	}

	span.SetAttributes(
		tracing.Attribute("image.ref", ref),
		tracing.Attribute("unpack", pullCtx.Unpack),
		tracing.Attribute("max.concurrent.downloads", pullCtx.MaxConcurrentDownloads),
		tracing.Attribute("platforms.count", len(pullCtx.Platforms)),
	)

	ctx, done, err := c.WithLease(ctx)
	if err != nil {
		return nil, err
	}
	defer done(ctx)

	var unpacker *unpack.Unpacker

	if pullCtx.Unpack {
		snapshotterName, err := c.resolveSnapshotterName(ctx, pullCtx.Snapshotter)
		if err != nil {
			return nil, fmt.Errorf("unable to resolve snapshotter: %w", err)
		}
		span.SetAttributes(tracing.Attribute("snapshotter.name", snapshotterName))
		var uconfig UnpackConfig
		for _, opt := range pullCtx.UnpackOpts {
			if err := opt(ctx, &uconfig); err != nil {
				return nil, err
			}
		}
		var platformMatcher platforms.Matcher
		if !uconfig.CheckPlatformSupported {
			platformMatcher = platforms.All
		}

		// Check client Unpack config
		platform := unpack.Platform{
			Platform:       platformMatcher,
			SnapshotterKey: snapshotterName,
			Snapshotter:    c.SnapshotService(snapshotterName),
			SnapshotOpts:   append(pullCtx.SnapshotterOpts, uconfig.SnapshotOpts...),
			Applier:        c.DiffService(),
			ApplyOpts:      uconfig.ApplyOpts,
		}
		uopts := []unpack.UnpackerOpt{unpack.WithUnpackPlatform(platform)}
		if pullCtx.MaxConcurrentDownloads > 0 {
			uopts = append(uopts, unpack.WithLimiter(semaphore.NewWeighted(int64(pullCtx.MaxConcurrentDownloads))))
		}
		if uconfig.DuplicationSuppressor != nil {
			uopts = append(uopts, unpack.WithDuplicationSuppressor(uconfig.DuplicationSuppressor))
		}
		unpacker, err = unpack.NewUnpacker(ctx, c.ContentStore(), uopts...)
		if err != nil {
			return nil, fmt.Errorf("unable to initialize unpacker: %w", err)
		}
		defer func() {
			if _, err := unpacker.Wait(); err != nil {
				if retErr == nil {
					retErr = fmt.Errorf("unpack: %w", err)
				}
			}
		}()
		wrapper := pullCtx.HandlerWrapper
		pullCtx.HandlerWrapper = func(h images.Handler) images.Handler {
			if wrapper == nil {
				return unpacker.Unpack(h)
			}
			return unpacker.Unpack(wrapper(h))
		}
	}

	img, err := c.fetch(ctx, pullCtx, ref, 1)
	if err != nil {
		return nil, err
	}

	// NOTE(fuweid): unpacker defers blobs download. before create image
	// record in ImageService, should wait for unpacking(including blobs
	// download).
	var ur unpack.Result
	if unpacker != nil {
		_, unpackSpan := tracing.StartSpan(ctx, tracing.Name(pullSpanPrefix, "UnpackWait"))
		if ur, err = unpacker.Wait(); err != nil {
			unpackSpan.SetStatus(err)
			unpackSpan.End()
			return nil, err
		}
		unpackSpan.End()
	}

	img, err = c.createNewImage(ctx, img)
	if err != nil {
		return nil, err
	}

	i := NewImageWithPlatform(c, img, pullCtx.PlatformMatcher)
	span.SetAttributes(tracing.Attribute("image.ref", i.Name()))

	if unpacker != nil && ur.Unpacks == 0 {
		// Unpack was tried previously but nothing was unpacked
		// This is at least required for schema 1 image.
		if err := i.Unpack(ctx, pullCtx.Snapshotter, pullCtx.UnpackOpts...); err != nil {
			return nil, fmt.Errorf("failed to unpack image on snapshotter %s: %w", pullCtx.Snapshotter, err)
		}
	}

	return i, nil
}

func (c *Client) fetch(ctx context.Context, rCtx *RemoteContext, ref string, limit int) (images.Image, error) {
	ctx, span := tracing.StartSpan(ctx, tracing.Name(pullSpanPrefix, "fetch"))
	defer span.End()
	store := c.ContentStore()
	name, desc, err := rCtx.Resolver.Resolve(ctx, ref)
	if err != nil {
		return images.Image{}, fmt.Errorf("failed to resolve reference %q: %w", ref, err)
	}

	fetcher, err := rCtx.Resolver.Fetcher(ctx, name)
	if err != nil {
		return images.Image{}, fmt.Errorf("failed to get fetcher for %q: %w", name, err)
	}

	var (
		handler images.Handler

		isConvertible         bool
		originalSchema1Digest string
		converterFunc         func(context.Context, ocispec.Descriptor) (ocispec.Descriptor, error)
		limiter               *semaphore.Weighted
	)

	if desc.MediaType == images.MediaTypeDockerSchema1Manifest && rCtx.ConvertSchema1 {
		schema1Converter := schema1.NewConverter(store, fetcher)

		handler = images.Handlers(append(rCtx.BaseHandlers, schema1Converter)...)

		isConvertible = true

		converterFunc = func(ctx context.Context, _ ocispec.Descriptor) (ocispec.Descriptor, error) {
			return schema1Converter.Convert(ctx)
		}

		originalSchema1Digest = desc.Digest.String()
	} else {
		// Get all the children for a descriptor
		childrenHandler := images.ChildrenHandler(store)
		// Set any children labels for that content
		childrenHandler = images.SetChildrenMappedLabels(store, childrenHandler, rCtx.ChildLabelMap)
		if rCtx.AllMetadata {
			// Filter manifests by platforms but allow to handle manifest
			// and configuration for not-target platforms
			childrenHandler = remotes.FilterManifestByPlatformHandler(childrenHandler, rCtx.PlatformMatcher)
		} else {
			// Filter children by platforms if specified.
			childrenHandler = images.FilterPlatforms(childrenHandler, rCtx.PlatformMatcher)
		}
		// Sort and limit manifests if a finite number is needed
		if limit > 0 {
			childrenHandler = images.LimitManifests(childrenHandler, rCtx.PlatformMatcher, limit)
		}

		// set isConvertible to true if there is application/octet-stream media type
		convertibleHandler := images.HandlerFunc(
			func(_ context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
				if desc.MediaType == docker.LegacyConfigMediaType {
					isConvertible = true
				}

				return []ocispec.Descriptor{}, nil
			},
		)

		appendDistSrcLabelHandler, err := docker.AppendDistributionSourceLabel(store, ref)
		if err != nil {
			return images.Image{}, err
		}

		handlers := append(rCtx.BaseHandlers,
			remotes.FetchHandler(store, fetcher),
			convertibleHandler,
			childrenHandler,
			appendDistSrcLabelHandler,
		)

		handler = images.Handlers(handlers...)

		converterFunc = func(ctx context.Context, desc ocispec.Descriptor) (ocispec.Descriptor, error) {
			return docker.ConvertManifest(ctx, store, desc)
		}
	}

	if rCtx.HandlerWrapper != nil {
		handler = rCtx.HandlerWrapper(handler)
	}

	if rCtx.MaxConcurrentDownloads > 0 {
		limiter = semaphore.NewWeighted(int64(rCtx.MaxConcurrentDownloads))
	}

	if err := images.Dispatch(ctx, handler, limiter, desc); err != nil {
		return images.Image{}, err
	}

	if isConvertible {
		if desc, err = converterFunc(ctx, desc); err != nil {
			return images.Image{}, err
		}
	}

	if originalSchema1Digest != "" {
		if rCtx.Labels == nil {
			rCtx.Labels = make(map[string]string)
		}
		rCtx.Labels[images.ConvertedDockerSchema1LabelKey] = originalSchema1Digest
	}

	return images.Image{
		Name:   name,
		Target: desc,
		Labels: rCtx.Labels,
	}, nil
}

func (c *Client) createNewImage(ctx context.Context, img images.Image) (images.Image, error) {
	ctx, span := tracing.StartSpan(ctx, tracing.Name(pullSpanPrefix, "pull.createNewImage"))
	defer span.End()
	is := c.ImageService()
	for {
		if created, err := is.Create(ctx, img); err != nil {
			if !errdefs.IsAlreadyExists(err) {
				return images.Image{}, err
			}

			updated, err := is.Update(ctx, img)
			if err != nil {
				// if image was removed, try create again
				if errdefs.IsNotFound(err) {
					continue
				}
				return images.Image{}, err
			}

			img = updated
		} else {
			img = created
		}

		return img, nil
	}
}
