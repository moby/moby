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

package images

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Image provides the model for how containerd views container images.
type Image struct {
	// Name of the image.
	//
	// To be pulled, it must be a reference compatible with resolvers.
	//
	// This field is required.
	Name string

	// Labels provide runtime decoration for the image record.
	//
	// There is no default behavior for how these labels are propagated. They
	// only decorate the static metadata object.
	//
	// This field is optional.
	Labels map[string]string

	// Target describes the root content for this image. Typically, this is
	// a manifest, index or manifest list.
	Target ocispec.Descriptor

	CreatedAt, UpdatedAt time.Time
}

// DeleteOptions provide options on image delete
type DeleteOptions struct {
	Synchronous bool
}

// DeleteOpt allows configuring a delete operation
type DeleteOpt func(context.Context, *DeleteOptions) error

// SynchronousDelete is used to indicate that an image deletion and removal of
// the image resources should occur synchronously before returning a result.
func SynchronousDelete() DeleteOpt {
	return func(ctx context.Context, o *DeleteOptions) error {
		o.Synchronous = true
		return nil
	}
}

// Store and interact with images
type Store interface {
	Get(ctx context.Context, name string) (Image, error)
	List(ctx context.Context, filters ...string) ([]Image, error)
	Create(ctx context.Context, image Image) (Image, error)

	// Update will replace the data in the store with the provided image. If
	// one or more fieldpaths are provided, only those fields will be updated.
	Update(ctx context.Context, image Image, fieldpaths ...string) (Image, error)

	Delete(ctx context.Context, name string, opts ...DeleteOpt) error
}

// TODO(stevvooe): Many of these functions make strong platform assumptions,
// which are untrue in a lot of cases. More refactoring must be done here to
// make this work in all cases.

// Config resolves the image configuration descriptor.
//
// The caller can then use the descriptor to resolve and process the
// configuration of the image.
func (image *Image) Config(ctx context.Context, provider content.Provider, platform platforms.MatchComparer) (ocispec.Descriptor, error) {
	return Config(ctx, provider, image.Target, platform)
}

// RootFS returns the unpacked diffids that make up and images rootfs.
//
// These are used to verify that a set of layers unpacked to the expected
// values.
func (image *Image) RootFS(ctx context.Context, provider content.Provider, platform platforms.MatchComparer) ([]digest.Digest, error) {
	desc, err := image.Config(ctx, provider, platform)
	if err != nil {
		return nil, err
	}
	return RootFS(ctx, provider, desc)
}

// Size returns the total size of an image's packed resources.
func (image *Image) Size(ctx context.Context, provider content.Provider, platform platforms.MatchComparer) (int64, error) {
	var size int64
	return size, Walk(ctx, Handlers(HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.Size < 0 {
			return nil, errors.Errorf("invalid size %v in %v (%v)", desc.Size, desc.Digest, desc.MediaType)
		}
		size += desc.Size
		return nil, nil
	}), LimitManifests(FilterPlatforms(ChildrenHandler(provider), platform), platform, 1)), image.Target)
}

type platformManifest struct {
	p *ocispec.Platform
	m *ocispec.Manifest
}

// Manifest resolves a manifest from the image for the given platform.
//
// When a manifest descriptor inside of a manifest index does not have
// a platform defined, the platform from the image config is considered.
//
// If the descriptor points to a non-index manifest, then the manifest is
// unmarshalled and returned without considering the platform inside of the
// config.
//
// TODO(stevvooe): This violates the current platform agnostic approach to this
// package by returning a specific manifest type. We'll need to refactor this
// to return a manifest descriptor or decide that we want to bring the API in
// this direction because this abstraction is not needed.`
func Manifest(ctx context.Context, provider content.Provider, image ocispec.Descriptor, platform platforms.MatchComparer) (ocispec.Manifest, error) {
	var (
		limit    = 1
		m        []platformManifest
		wasIndex bool
	)

	if err := Walk(ctx, HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		switch desc.MediaType {
		case MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			var manifest ocispec.Manifest
			if err := json.Unmarshal(p, &manifest); err != nil {
				return nil, err
			}

			if desc.Digest != image.Digest && platform != nil {
				if desc.Platform != nil && !platform.Match(*desc.Platform) {
					return nil, nil
				}

				if desc.Platform == nil {
					p, err := content.ReadBlob(ctx, provider, manifest.Config)
					if err != nil {
						return nil, err
					}

					var image ocispec.Image
					if err := json.Unmarshal(p, &image); err != nil {
						return nil, err
					}

					if !platform.Match(platforms.Normalize(ocispec.Platform{OS: image.OS, Architecture: image.Architecture})) {
						return nil, nil
					}

				}
			}

			m = append(m, platformManifest{
				p: desc.Platform,
				m: &manifest,
			})

			return nil, nil
		case MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			var idx ocispec.Index
			if err := json.Unmarshal(p, &idx); err != nil {
				return nil, err
			}

			if platform == nil {
				return idx.Manifests, nil
			}

			var descs []ocispec.Descriptor
			for _, d := range idx.Manifests {
				if d.Platform == nil || platform.Match(*d.Platform) {
					descs = append(descs, d)
				}
			}

			sort.SliceStable(descs, func(i, j int) bool {
				if descs[i].Platform == nil {
					return false
				}
				if descs[j].Platform == nil {
					return true
				}
				return platform.Less(*descs[i].Platform, *descs[j].Platform)
			})

			wasIndex = true

			if len(descs) > limit {
				return descs[:limit], nil
			}
			return descs, nil
		}
		return nil, errors.Wrapf(errdefs.ErrNotFound, "unexpected media type %v for %v", desc.MediaType, desc.Digest)
	}), image); err != nil {
		return ocispec.Manifest{}, err
	}

	if len(m) == 0 {
		err := errors.Wrapf(errdefs.ErrNotFound, "manifest %v", image.Digest)
		if wasIndex {
			err = errors.Wrapf(errdefs.ErrNotFound, "no match for platform in manifest %v", image.Digest)
		}
		return ocispec.Manifest{}, err
	}
	return *m[0].m, nil
}

// Config resolves the image configuration descriptor using a content provided
// to resolve child resources on the image.
//
// The caller can then use the descriptor to resolve and process the
// configuration of the image.
func Config(ctx context.Context, provider content.Provider, image ocispec.Descriptor, platform platforms.MatchComparer) (ocispec.Descriptor, error) {
	manifest, err := Manifest(ctx, provider, image, platform)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return manifest.Config, err
}

// Platforms returns one or more platforms supported by the image.
func Platforms(ctx context.Context, provider content.Provider, image ocispec.Descriptor) ([]ocispec.Platform, error) {
	var platformSpecs []ocispec.Platform
	return platformSpecs, Walk(ctx, Handlers(HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.Platform != nil {
			platformSpecs = append(platformSpecs, *desc.Platform)
			return nil, ErrSkipDesc
		}

		switch desc.MediaType {
		case MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			var image ocispec.Image
			if err := json.Unmarshal(p, &image); err != nil {
				return nil, err
			}

			platformSpecs = append(platformSpecs,
				platforms.Normalize(ocispec.Platform{OS: image.OS, Architecture: image.Architecture}))
		}
		return nil, nil
	}), ChildrenHandler(provider)), image)
}

// Check returns nil if the all components of an image are available in the
// provider for the specified platform.
//
// If available is true, the caller can assume that required represents the
// complete set of content required for the image.
//
// missing will have the components that are part of required but not avaiiable
// in the provider.
//
// If there is a problem resolving content, an error will be returned.
func Check(ctx context.Context, provider content.Provider, image ocispec.Descriptor, platform platforms.MatchComparer) (available bool, required, present, missing []ocispec.Descriptor, err error) {
	mfst, err := Manifest(ctx, provider, image, platform)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, []ocispec.Descriptor{image}, nil, []ocispec.Descriptor{image}, nil
		}

		return false, nil, nil, nil, errors.Wrapf(err, "failed to check image %v", image.Digest)
	}

	// TODO(stevvooe): It is possible that referenced conponents could have
	// children, but this is rare. For now, we ignore this and only verify
	// that manifest components are present.
	required = append([]ocispec.Descriptor{mfst.Config}, mfst.Layers...)

	for _, desc := range required {
		ra, err := provider.ReaderAt(ctx, desc)
		if err != nil {
			if errdefs.IsNotFound(err) {
				missing = append(missing, desc)
				continue
			} else {
				return false, nil, nil, nil, errors.Wrapf(err, "failed to check image %v", desc.Digest)
			}
		}
		ra.Close()
		present = append(present, desc)

	}

	return true, required, present, missing, nil
}

// Children returns the immediate children of content described by the descriptor.
func Children(ctx context.Context, provider content.Provider, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	var descs []ocispec.Descriptor
	switch desc.MediaType {
	case MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
		p, err := content.ReadBlob(ctx, provider, desc)
		if err != nil {
			return nil, err
		}

		// TODO(stevvooe): We just assume oci manifest, for now. There may be
		// subtle differences from the docker version.
		var manifest ocispec.Manifest
		if err := json.Unmarshal(p, &manifest); err != nil {
			return nil, err
		}

		descs = append(descs, manifest.Config)
		descs = append(descs, manifest.Layers...)
	case MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
		p, err := content.ReadBlob(ctx, provider, desc)
		if err != nil {
			return nil, err
		}

		var index ocispec.Index
		if err := json.Unmarshal(p, &index); err != nil {
			return nil, err
		}

		descs = append(descs, index.Manifests...)
	default:
		if IsLayerType(desc.MediaType) || IsKnownConfig(desc.MediaType) {
			// childless data types.
			return nil, nil
		}
		log.G(ctx).Debugf("encountered unknown type %v; children may not be fetched", desc.MediaType)
	}

	return descs, nil
}

// RootFS returns the unpacked diffids that make up and images rootfs.
//
// These are used to verify that a set of layers unpacked to the expected
// values.
func RootFS(ctx context.Context, provider content.Provider, configDesc ocispec.Descriptor) ([]digest.Digest, error) {
	p, err := content.ReadBlob(ctx, provider, configDesc)
	if err != nil {
		return nil, err
	}

	var config ocispec.Image
	if err := json.Unmarshal(p, &config); err != nil {
		return nil, err
	}
	return config.RootFS.DiffIDs, nil
}
