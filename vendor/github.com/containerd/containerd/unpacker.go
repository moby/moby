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
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
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
	var config UnpackConfig
	for _, o := range rCtx.UnpackOpts {
		if err := o(ctx, &config); err != nil {
			return nil, err
		}
	}
	return &unpacker{
		updateCh:    make(chan ocispec.Descriptor, 128),
		snapshotter: snapshotter,
		config:      config,
		c:           c,
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
		return errors.Wrap(err, "unmarshal image config")
	}
	diffIDs := i.RootFS.DiffIDs
	if len(layers) != len(diffIDs) {
		return errors.Errorf("number of layers and diffIDs don't match: %d != %d", len(layers), len(diffIDs))
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

EachLayer:
	for i, desc := range layers {
		parent := identity.ChainID(chain)
		chain = append(chain, diffIDs[i])

		chainID := identity.ChainID(chain).String()
		if _, err := sn.Stat(ctx, chainID); err == nil {
			// no need to handle
			continue
		} else if !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to stat snapshot %s", chainID)
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
			key = fmt.Sprintf("extract-%s %s", uniquePart(), chainID)
			mounts, err = sn.Prepare(ctx, key, parent.String(), opts...)
			if err != nil {
				if errdefs.IsAlreadyExists(err) {
					if _, err := sn.Stat(ctx, chainID); err != nil {
						if !errdefs.IsNotFound(err) {
							return errors.Wrapf(err, "failed to stat snapshot %s", chainID)
						}
						// Try again, this should be rare, log it
						log.G(ctx).WithField("key", key).WithField("chainid", chainID).Debug("extraction snapshot already exists, chain id not found")
					} else {
						// no need to handle, snapshot now found with chain id
						continue EachLayer
					}
				} else {
					return errors.Wrapf(err, "failed to prepare extraction snapshot %q", key)
				}
			} else {
				break
			}
		}
		if err != nil {
			return errors.Wrap(err, "unable to prepare extraction snapshot")
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

			go func() {
				err := u.fetch(ctx, h, layers[i:], fetchC)
				if err != nil {
					fetchErr <- err
				}
				close(fetchErr)
			}()
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
			return errors.Wrapf(err, "failed to extract layer %s", diffIDs[i])
		}
		if diff.Digest != diffIDs[i] {
			abort()
			return errors.Errorf("wrong diff id calculated on extraction %q", diffIDs[i])
		}

		if err = sn.Commit(ctx, chainID, key, opts...); err != nil {
			abort()
			if errdefs.IsAlreadyExists(err) {
				continue
			}
			return errors.Wrapf(err, "failed to commit snapshot %s", key)
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

		if u.limiter != nil {
			if err := u.limiter.Acquire(ctx, 1); err != nil {
				return err
			}
		}

		eg.Go(func() error {
			_, err := h.Handle(ctx2, desc)
			if u.limiter != nil {
				u.limiter.Release(1)
			}
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
			children, err := f.Handle(ctx, desc)
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

func uniquePart() string {
	t := time.Now()
	var b [3]byte
	// Ignore read failures, just decreases uniqueness
	rand.Read(b[:])
	return fmt.Sprintf("%d-%s", t.Nanosecond(), base64.URLEncoding.EncodeToString(b[:]))
}
