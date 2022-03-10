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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/semaphore"
)

// Image describes an image used by containers
type Image interface {
	// Name of the image
	Name() string
	// Target descriptor for the image content
	Target() ocispec.Descriptor
	// Labels of the image
	Labels() map[string]string
	// Unpack unpacks the image's content into a snapshot
	Unpack(context.Context, string, ...UnpackOpt) error
	// RootFS returns the unpacked diffids that make up images rootfs.
	RootFS(ctx context.Context) ([]digest.Digest, error)
	// Size returns the total size of the image's packed resources.
	Size(ctx context.Context) (int64, error)
	// Usage returns a usage calculation for the image.
	Usage(context.Context, ...UsageOpt) (int64, error)
	// Config descriptor for the image.
	Config(ctx context.Context) (ocispec.Descriptor, error)
	// IsUnpacked returns whether or not an image is unpacked.
	IsUnpacked(context.Context, string) (bool, error)
	// ContentStore provides a content store which contains image blob data
	ContentStore() content.Store
	// Metadata returns the underlying image metadata
	Metadata() images.Image
	// Platform returns the platform match comparer. Can be nil.
	Platform() platforms.MatchComparer
}

type usageOptions struct {
	manifestLimit *int
	manifestOnly  bool
	snapshots     bool
}

// UsageOpt is used to configure the usage calculation
type UsageOpt func(*usageOptions) error

// WithUsageManifestLimit sets the limit to the number of manifests which will
// be walked for usage. Setting this value to 0 will require all manifests to
// be walked, returning ErrNotFound if manifests are missing.
// NOTE: By default all manifests which exist will be walked
// and any non-existent manifests and their subobjects will be ignored.
func WithUsageManifestLimit(i int) UsageOpt {
	// If 0 then don't filter any manifests
	// By default limits to current platform
	return func(o *usageOptions) error {
		o.manifestLimit = &i
		return nil
	}
}

// WithSnapshotUsage will check for referenced snapshots from the image objects
// and include the snapshot size in the total usage.
func WithSnapshotUsage() UsageOpt {
	return func(o *usageOptions) error {
		o.snapshots = true
		return nil
	}
}

// WithManifestUsage is used to get the usage for an image based on what is
// reported by the manifests rather than what exists in the content store.
// NOTE: This function is best used with the manifest limit set to get a
// consistent value, otherwise non-existent manifests will be excluded.
func WithManifestUsage() UsageOpt {
	return func(o *usageOptions) error {
		o.manifestOnly = true
		return nil
	}
}

var _ = (Image)(&image{})

// NewImage returns a client image object from the metadata image
func NewImage(client *Client, i images.Image) Image {
	return &image{
		client:   client,
		i:        i,
		platform: client.platform,
	}
}

// NewImageWithPlatform returns a client image object from the metadata image
func NewImageWithPlatform(client *Client, i images.Image, platform platforms.MatchComparer) Image {
	return &image{
		client:   client,
		i:        i,
		platform: platform,
	}
}

type image struct {
	client *Client

	i        images.Image
	platform platforms.MatchComparer
}

func (i *image) Metadata() images.Image {
	return i.i
}

func (i *image) Name() string {
	return i.i.Name
}

func (i *image) Target() ocispec.Descriptor {
	return i.i.Target
}

func (i *image) Labels() map[string]string {
	return i.i.Labels
}

func (i *image) RootFS(ctx context.Context) ([]digest.Digest, error) {
	provider := i.client.ContentStore()
	return i.i.RootFS(ctx, provider, i.platform)
}

func (i *image) Size(ctx context.Context) (int64, error) {
	return i.Usage(ctx, WithUsageManifestLimit(1), WithManifestUsage())
}

func (i *image) Usage(ctx context.Context, opts ...UsageOpt) (int64, error) {
	var config usageOptions
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return 0, err
		}
	}

	var (
		provider  = i.client.ContentStore()
		handler   = images.ChildrenHandler(provider)
		size      int64
		mustExist bool
	)

	if config.manifestLimit != nil {
		handler = images.LimitManifests(handler, i.platform, *config.manifestLimit)
		mustExist = true
	}

	var wh images.HandlerFunc = func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		var usage int64
		children, err := handler(ctx, desc)
		if err != nil {
			if !errdefs.IsNotFound(err) || mustExist {
				return nil, err
			}
			if !config.manifestOnly {
				// Do not count size of non-existent objects
				desc.Size = 0
			}
		} else if config.snapshots || !config.manifestOnly {
			info, err := provider.Info(ctx, desc.Digest)
			if err != nil {
				if !errdefs.IsNotFound(err) {
					return nil, err
				}
				if !config.manifestOnly {
					// Do not count size of non-existent objects
					desc.Size = 0
				}
			} else if info.Size > desc.Size {
				// Count actual usage, Size may be unset or -1
				desc.Size = info.Size
			}

			if config.snapshots {
				for k, v := range info.Labels {
					const prefix = "containerd.io/gc.ref.snapshot."
					if !strings.HasPrefix(k, prefix) {
						continue
					}

					sn := i.client.SnapshotService(k[len(prefix):])
					if sn == nil {
						continue
					}

					u, err := sn.Usage(ctx, v)
					if err != nil {
						if !errdefs.IsNotFound(err) && !errdefs.IsInvalidArgument(err) {
							return nil, err
						}
					} else {
						usage += u.Size
					}
				}
			}
		}

		// Ignore unknown sizes. Generally unknown sizes should
		// never be set in manifests, however, the usage
		// calculation does not need to enforce this.
		if desc.Size >= 0 {
			usage += desc.Size
		}

		atomic.AddInt64(&size, usage)

		return children, nil
	}

	l := semaphore.NewWeighted(3)
	if err := images.Dispatch(ctx, wh, l, i.i.Target); err != nil {
		return 0, err
	}

	return size, nil
}

func (i *image) Config(ctx context.Context) (ocispec.Descriptor, error) {
	provider := i.client.ContentStore()
	return i.i.Config(ctx, provider, i.platform)
}

func (i *image) IsUnpacked(ctx context.Context, snapshotterName string) (bool, error) {
	sn, err := i.client.getSnapshotter(ctx, snapshotterName)
	if err != nil {
		return false, err
	}
	cs := i.client.ContentStore()

	diffs, err := i.i.RootFS(ctx, cs, i.platform)
	if err != nil {
		return false, err
	}

	chainID := identity.ChainID(diffs)
	_, err = sn.Stat(ctx, chainID.String())
	if err == nil {
		return true, nil
	} else if !errdefs.IsNotFound(err) {
		return false, err
	}

	return false, nil
}

// UnpackConfig provides configuration for the unpack of an image
type UnpackConfig struct {
	// ApplyOpts for applying a diff to a snapshotter
	ApplyOpts []diff.ApplyOpt
	// SnapshotOpts for configuring a snapshotter
	SnapshotOpts []snapshots.Opt
	// CheckPlatformSupported is whether to validate that a snapshotter
	// supports an image's platform before unpacking
	CheckPlatformSupported bool
}

// UnpackOpt provides configuration for unpack
type UnpackOpt func(context.Context, *UnpackConfig) error

// WithSnapshotterPlatformCheck sets `CheckPlatformSupported` on the UnpackConfig
func WithSnapshotterPlatformCheck() UnpackOpt {
	return func(ctx context.Context, uc *UnpackConfig) error {
		uc.CheckPlatformSupported = true
		return nil
	}
}

func (i *image) Unpack(ctx context.Context, snapshotterName string, opts ...UnpackOpt) error {
	ctx, done, err := i.client.WithLease(ctx)
	if err != nil {
		return err
	}
	defer done(ctx)

	var config UnpackConfig
	for _, o := range opts {
		if err := o(ctx, &config); err != nil {
			return err
		}
	}

	manifest, err := i.getManifest(ctx, i.platform)
	if err != nil {
		return err
	}

	layers, err := i.getLayers(ctx, i.platform, manifest)
	if err != nil {
		return err
	}

	var (
		a  = i.client.DiffService()
		cs = i.client.ContentStore()

		chain    []digest.Digest
		unpacked bool
	)
	snapshotterName, err = i.client.resolveSnapshotterName(ctx, snapshotterName)
	if err != nil {
		return err
	}
	sn, err := i.client.getSnapshotter(ctx, snapshotterName)
	if err != nil {
		return err
	}
	if config.CheckPlatformSupported {
		if err := i.checkSnapshotterSupport(ctx, snapshotterName, manifest); err != nil {
			return err
		}
	}

	for _, layer := range layers {
		unpacked, err = rootfs.ApplyLayerWithOpts(ctx, layer, chain, sn, a, config.SnapshotOpts, config.ApplyOpts)
		if err != nil {
			return err
		}

		if unpacked {
			// Set the uncompressed label after the uncompressed
			// digest has been verified through apply.
			cinfo := content.Info{
				Digest: layer.Blob.Digest,
				Labels: map[string]string{
					"containerd.io/uncompressed": layer.Diff.Digest.String(),
				},
			}
			if _, err := cs.Update(ctx, cinfo, "labels.containerd.io/uncompressed"); err != nil {
				return err
			}
		}

		chain = append(chain, layer.Diff.Digest)
	}

	desc, err := i.i.Config(ctx, cs, i.platform)
	if err != nil {
		return err
	}

	rootfs := identity.ChainID(chain).String()

	cinfo := content.Info{
		Digest: desc.Digest,
		Labels: map[string]string{
			fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", snapshotterName): rootfs,
		},
	}

	_, err = cs.Update(ctx, cinfo, fmt.Sprintf("labels.containerd.io/gc.ref.snapshot.%s", snapshotterName))
	return err
}

func (i *image) getManifest(ctx context.Context, platform platforms.MatchComparer) (ocispec.Manifest, error) {
	cs := i.ContentStore()
	manifest, err := images.Manifest(ctx, cs, i.i.Target, platform)
	if err != nil {
		return ocispec.Manifest{}, err
	}
	return manifest, nil
}

func (i *image) getLayers(ctx context.Context, platform platforms.MatchComparer, manifest ocispec.Manifest) ([]rootfs.Layer, error) {
	cs := i.ContentStore()
	diffIDs, err := i.i.RootFS(ctx, cs, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve rootfs: %w", err)
	}
	if len(diffIDs) != len(manifest.Layers) {
		return nil, errors.New("mismatched image rootfs and manifest layers")
	}
	layers := make([]rootfs.Layer, len(diffIDs))
	for i := range diffIDs {
		layers[i].Diff = ocispec.Descriptor{
			// TODO: derive media type from compressed type
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    diffIDs[i],
		}
		layers[i].Blob = manifest.Layers[i]
	}
	return layers, nil
}

func (i *image) getManifestPlatform(ctx context.Context, manifest ocispec.Manifest) (ocispec.Platform, error) {
	cs := i.ContentStore()
	p, err := content.ReadBlob(ctx, cs, manifest.Config)
	if err != nil {
		return ocispec.Platform{}, err
	}

	var image ocispec.Image
	if err := json.Unmarshal(p, &image); err != nil {
		return ocispec.Platform{}, err
	}
	return platforms.Normalize(ocispec.Platform{OS: image.OS, Architecture: image.Architecture}), nil
}

func (i *image) checkSnapshotterSupport(ctx context.Context, snapshotterName string, manifest ocispec.Manifest) error {
	snapshotterPlatformMatcher, err := i.client.GetSnapshotterSupportedPlatforms(ctx, snapshotterName)
	if err != nil {
		return err
	}

	manifestPlatform, err := i.getManifestPlatform(ctx, manifest)
	if err != nil {
		return err
	}

	if snapshotterPlatformMatcher.Match(manifestPlatform) {
		return nil
	}
	return fmt.Errorf("snapshotter %s does not support platform %s for image %s", snapshotterName, manifestPlatform, manifest.Config.Digest)
}

func (i *image) ContentStore() content.Store {
	return i.client.ContentStore()
}

func (i *image) Platform() platforms.MatchComparer {
	return i.platform
}
