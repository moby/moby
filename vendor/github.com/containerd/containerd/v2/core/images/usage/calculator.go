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

package usage

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"golang.org/x/sync/semaphore"
)

type usageOptions struct {
	platform      platforms.MatchComparer
	manifestLimit int
	manifestOnly  bool
	snapshots     func(name string) snapshots.Snapshotter
}

type Opt func(*usageOptions) error

// WithManifestLimit sets the limit to the number of manifests which will
// be walked for usage. Setting this value to 0 will require all manifests to
// be walked, returning ErrNotFound if manifests are missing.
// NOTE: By default all manifests which exist will be walked
// and any non-existent manifests and their subobjects will be ignored.
func WithManifestLimit(platform platforms.MatchComparer, i int) Opt {
	// If 0 then don't filter any manifests
	// By default limits to current platform
	return func(o *usageOptions) error {
		o.manifestLimit = i
		o.platform = platform
		return nil
	}
}

// WithSnapshotters will check for referenced snapshots from the image objects
// and include the snapshot size in the total usage.
func WithSnapshotters(f func(string) snapshots.Snapshotter) Opt {
	return func(o *usageOptions) error {
		o.snapshots = f
		return nil
	}
}

// WithManifestUsage is used to get the usage for an image based on what is
// reported by the manifests rather than what exists in the content store.
// NOTE: This function is best used with the manifest limit set to get a
// consistent value, otherwise non-existent manifests will be excluded.
func WithManifestUsage() Opt {
	return func(o *usageOptions) error {
		o.manifestOnly = true
		return nil
	}
}

func CalculateImageUsage(ctx context.Context, i images.Image, provider content.InfoReaderProvider, opts ...Opt) (int64, error) {
	var config usageOptions
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return 0, err
		}
	}

	var (
		handler   = images.ChildrenHandler(provider)
		size      int64
		mustExist bool
	)

	if config.platform != nil {
		handler = images.LimitManifests(handler, config.platform, config.manifestLimit)
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
		} else if config.snapshots != nil || !config.manifestOnly {
			info, err := provider.Info(ctx, desc.Digest)
			if err != nil {
				if !errdefs.IsNotFound(err) {
					return nil, err
				}
				if !config.manifestOnly {
					// Do not count size of non-existent objects
					desc.Size = 0
				}
			} else {
				if info.Size > desc.Size {
					// Count actual usage, Size may be unset or -1
					desc.Size = info.Size
				}

				if config.snapshots != nil {
					for k, v := range info.Labels {
						const prefix = "containerd.io/gc.ref.snapshot."
						if !strings.HasPrefix(k, prefix) {
							continue
						}

						sn := config.snapshots(k[len(prefix):])
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
	if err := images.Dispatch(ctx, wh, l, i.Target); err != nil {
		return 0, err
	}

	return size, nil
}
