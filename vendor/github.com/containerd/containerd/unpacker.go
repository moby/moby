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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/kmutex"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	labelSnapshotRef = "containerd.io/snapshot.ref"
)

type unpacker struct {
	updateCh    chan ocispec.Descriptor
	snapshotter string
	config      UnpackConfig
	c           *Client
	limiter     *semaphore.Weighted
}

func (c *Client) newUnpacker(ctx context.Context, rCtx *RemoteContext) (*unpacker, error) {
	snapshotter, err := c.resolveSnapshotterName(ctx, rCtx.Snapshotter)
	if err != nil {
		return nil, err
	}
	var config = UnpackConfig{
		DuplicationSuppressor: kmutex.NewNoop(),
	}
	for _, o := range rCtx.UnpackOpts {
		if err := o(ctx, &config); err != nil {
			return nil, err
		}
	}
	var limiter *semaphore.Weighted
	if rCtx.MaxConcurrentDownloads > 0 {
		limiter = semaphore.NewWeighted(int64(rCtx.MaxConcurrentDownloads))
	}
	return &unpacker{
		updateCh:    make(chan ocispec.Descriptor, 128),
		snapshotter: snapshotter,
		config:      config,
		c:           c,
		limiter:     limiter,
	}, nil
}

func (u *unpacker) unpack(
	ctx context.Context,
	rCtx *RemoteContext,
	h images.Handler,
	config ocispec.Descriptor,
	layers []ocispec.Descriptor,
) error {
	p, err := content.ReadBlob(ctx, u.c.ContentStore(), config)
	if err != nil {
		return err
	}

	var i ocispec.Image
	if err := json.Unmarshal(p, &i); err != nil {
		return fmt.Errorf("unmarshal image config: %w", err)
	}
	diffIDs := i.RootFS.DiffIDs
	if len(layers) != len(diffIDs) {
		return fmt.Errorf("number of layers and diffIDs don't match: %d != %d", len(layers), len(diffIDs))
	}

	if u.config.CheckPlatformSupported {
		imgPlatform := platforms.Normalize(ocispec.Platform{OS: i.OS, Architecture: i.Architecture})
		snapshotterPlatformMatcher, err := u.c.GetSnapshotterSupportedPlatforms(ctx, u.snapshotter)
		if err != nil {
			return fmt.Errorf("failed to find supported platforms for snapshotter %s: %w", u.snapshotter, err)
		}
		if !snapshotterPlatformMatcher.Match(imgPlatform) {
			return fmt.Errorf("snapshotter %s does not support platform %s for image %s", u.snapshotter, imgPlatform, config.Digest)
		}
	}

	var (
		sn = u.c.SnapshotService(u.snapshotter)
		a  = u.c.DiffService()
		cs = u.c.ContentStore()

		chain []digest.Digest

		fetchOffset int
		fetchC      []chan struct{}
		fetchErr    chan error
	)

	// If there is an early return, ensure any ongoing
	// fetches get their context cancelled
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	doUnpackFn := func(i int, desc ocispec.Descriptor) error {
		parent := identity.ChainID(chain)
		chain = append(chain, diffIDs[i])
		chainID := identity.ChainID(chain).String()

		unlock, err := u.lockSnChainID(ctx, chainID)
		if err != nil {
			return err
		}
		defer unlock()

		if _, err := sn.Stat(ctx, chainID); err == nil {
			// no need to handle
			return nil
		} else if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to stat snapshot %s: %w", chainID, err)
		}

		// inherits annotations which are provided as snapshot labels.
		labels := snapshots.FilterInheritedLabels(desc.Annotations)
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[labelSnapshotRef] = chainID

		var (
			key    string
			mounts []mount.Mount
			opts   = append(rCtx.SnapshotterOpts, snapshots.WithLabels(labels))
		)

		for try := 1; try <= 3; try++ {
			// Prepare snapshot with from parent, label as root
			key = fmt.Sprintf(snapshots.UnpackKeyFormat, uniquePart(), chainID)
			mounts, err = sn.Prepare(ctx, key, parent.String(), opts...)
			if err != nil {
				if errdefs.IsAlreadyExists(err) {
					if _, err := sn.Stat(ctx, chainID); err != nil {
						if !errdefs.IsNotFound(err) {
							return fmt.Errorf("failed to stat snapshot %s: %w", chainID, err)
						}
						// Try again, this should be rare, log it
						log.G(ctx).WithField("key", key).WithField("chainid", chainID).Debug("extraction snapshot already exists, chain id not found")
					} else {
						// no need to handle, snapshot now found with chain id
						return nil
					}
				} else {
					return fmt.Errorf("failed to prepare extraction snapshot %q: %w", key, err)
				}
			} else {
				break
			}
		}
		if err != nil {
			return fmt.Errorf("unable to prepare extraction snapshot: %w", err)
		}

		// Abort the snapshot if commit does not happen
		abort := func() {
			if err := sn.Remove(ctx, key); err != nil {
				log.G(ctx).WithError(err).Errorf("failed to cleanup %q", key)
			}
		}

		if fetchErr == nil {
			fetchErr = make(chan error, 1)
			fetchOffset = i
			fetchC = make([]chan struct{}, len(layers)-fetchOffset)
			for i := range fetchC {
				fetchC[i] = make(chan struct{})
			}

			go func(i int) {
				err := u.fetch(ctx, h, layers[i:], fetchC)
				if err != nil {
					fetchErr <- err
				}
				close(fetchErr)
			}(i)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-fetchErr:
			if err != nil {
				return err
			}
		case <-fetchC[i-fetchOffset]:
		}

		diff, err := a.Apply(ctx, desc, mounts, u.config.ApplyOpts...)
		if err != nil {
			abort()
			return fmt.Errorf("failed to extract layer %s: %w", diffIDs[i], err)
		}
		if diff.Digest != diffIDs[i] {
			abort()
			return fmt.Errorf("wrong diff id calculated on extraction %q", diffIDs[i])
		}

		if err = sn.Commit(ctx, chainID, key, opts...); err != nil {
			abort()
			if errdefs.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("failed to commit snapshot %s: %w", key, err)
		}

		// Set the uncompressed label after the uncompressed
		// digest has been verified through apply.
		cinfo := content.Info{
			Digest: desc.Digest,
			Labels: map[string]string{
				"containerd.io/uncompressed": diff.Digest.String(),
			},
		}
		if _, err := cs.Update(ctx, cinfo, "labels.containerd.io/uncompressed"); err != nil {
			return err
		}
		return nil
	}

	for i, desc := range layers {
		if err := doUnpackFn(i, desc); err != nil {
			return err
		}
	}

	chainID := identity.ChainID(chain).String()
	cinfo := content.Info{
		Digest: config.Digest,
		Labels: map[string]string{
			fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", u.snapshotter): chainID,
		},
	}
	_, err = cs.Update(ctx, cinfo, fmt.Sprintf("labels.containerd.io/gc.ref.snapshot.%s", u.snapshotter))
	if err != nil {
		return err
	}
	log.G(ctx).WithFields(logrus.Fields{
		"config":  config.Digest,
		"chainID": chainID,
	}).Debug("image unpacked")

	return nil
}

func (u *unpacker) fetch(ctx context.Context, h images.Handler, layers []ocispec.Descriptor, done []chan struct{}) error {
	eg, ctx2 := errgroup.WithContext(ctx)
	for i, desc := range layers {
		desc := desc
		i := i

		if err := u.acquire(ctx); err != nil {
			return err
		}

		eg.Go(func() error {
			unlock, err := u.lockBlobDescriptor(ctx2, desc)
			if err != nil {
				u.release()
				return err
			}

			_, err = h.Handle(ctx2, desc)

			unlock()
			u.release()

			if err != nil && !errors.Is(err, images.ErrSkipDesc) {
				return err
			}
			close(done[i])

			return nil
		})
	}

	return eg.Wait()
}

func (u *unpacker) handlerWrapper(
	uctx context.Context,
	rCtx *RemoteContext,
	unpacks *int32,
) (func(images.Handler) images.Handler, *errgroup.Group) {
	eg, uctx := errgroup.WithContext(uctx)
	return func(f images.Handler) images.Handler {
		var (
			lock   sync.Mutex
			layers = map[digest.Digest][]ocispec.Descriptor{}
		)
		return images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			unlock, err := u.lockBlobDescriptor(ctx, desc)
			if err != nil {
				return nil, err
			}

			children, err := f.Handle(ctx, desc)
			unlock()
			if err != nil {
				return children, err
			}

			switch desc.MediaType {
			case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
				var nonLayers []ocispec.Descriptor
				var manifestLayers []ocispec.Descriptor

				// Split layers from non-layers, layers will be handled after
				// the config
				for _, child := range children {
					if images.IsLayerType(child.MediaType) {
						manifestLayers = append(manifestLayers, child)
					} else {
						nonLayers = append(nonLayers, child)
					}
				}

				lock.Lock()
				for _, nl := range nonLayers {
					layers[nl.Digest] = manifestLayers
				}
				lock.Unlock()

				children = nonLayers
			case images.MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig:
				lock.Lock()
				l := layers[desc.Digest]
				lock.Unlock()
				if len(l) > 0 {
					atomic.AddInt32(unpacks, 1)
					eg.Go(func() error {
						return u.unpack(uctx, rCtx, f, desc, l)
					})
				}
			}
			return children, nil
		})
	}, eg
}

func (u *unpacker) acquire(ctx context.Context) error {
	if u.limiter == nil {
		return nil
	}
	return u.limiter.Acquire(ctx, 1)
}

func (u *unpacker) release() {
	if u.limiter == nil {
		return
	}
	u.limiter.Release(1)
}

func (u *unpacker) lockSnChainID(ctx context.Context, chainID string) (func(), error) {
	key := u.makeChainIDKeyWithSnapshotter(chainID)

	if err := u.config.DuplicationSuppressor.Lock(ctx, key); err != nil {
		return nil, err
	}
	return func() {
		u.config.DuplicationSuppressor.Unlock(key)
	}, nil
}

func (u *unpacker) lockBlobDescriptor(ctx context.Context, desc ocispec.Descriptor) (func(), error) {
	key := u.makeBlobDescriptorKey(desc)

	if err := u.config.DuplicationSuppressor.Lock(ctx, key); err != nil {
		return nil, err
	}
	return func() {
		u.config.DuplicationSuppressor.Unlock(key)
	}, nil
}

func (u *unpacker) makeChainIDKeyWithSnapshotter(chainID string) string {
	return fmt.Sprintf("sn://%s/%v", u.snapshotter, chainID)
}

func (u *unpacker) makeBlobDescriptorKey(desc ocispec.Descriptor) string {
	return fmt.Sprintf("blob://%v", desc.Digest)
}

func uniquePart() string {
	t := time.Now()
	var b [3]byte
	// Ignore read failures, just decreases uniqueness
	rand.Read(b[:])
	return fmt.Sprintf("%d-%s", t.Nanosecond(), base64.URLEncoding.EncodeToString(b[:]))
}
