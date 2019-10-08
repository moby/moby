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
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/rootfs"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type layerState struct {
	layer      rootfs.Layer
	downloaded bool
	unpacked   bool
}

type unpacker struct {
	updateCh    chan ocispec.Descriptor
	snapshotter string
	config      UnpackConfig
	c           *Client
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

func (u *unpacker) unpack(ctx context.Context, config ocispec.Descriptor, layers []ocispec.Descriptor) error {
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

		states []layerState
		chain  []digest.Digest
	)
	for i, desc := range layers {
		states = append(states, layerState{
			layer: rootfs.Layer{
				Blob: desc,
				Diff: ocispec.Descriptor{
					MediaType: ocispec.MediaTypeImageLayer,
					Digest:    diffIDs[i],
				},
			},
		})
	}
	for {
		var layer ocispec.Descriptor
		select {
		case layer = <-u.updateCh:
		case <-ctx.Done():
			return ctx.Err()
		}
		log.G(ctx).WithField("desc", layer).Debug("layer downloaded")
		for i := range states {
			if states[i].layer.Blob.Digest != layer.Digest {
				continue
			}
			// Different layers may have the same digest. When that
			// happens, we should continue marking the next layer
			// as downloaded.
			if states[i].downloaded {
				continue
			}
			states[i].downloaded = true
			break
		}
		for i := range states {
			if !states[i].downloaded {
				break
			}
			if states[i].unpacked {
				continue
			}

			log.G(ctx).WithFields(logrus.Fields{
				"desc": states[i].layer.Blob,
				"diff": states[i].layer.Diff,
			}).Debug("unpack layer")

			unpacked, err := rootfs.ApplyLayerWithOpts(ctx, states[i].layer, chain, sn, a,
				u.config.SnapshotOpts, u.config.ApplyOpts)
			if err != nil {
				return err
			}

			if unpacked {
				// Set the uncompressed label after the uncompressed
				// digest has been verified through apply.
				cinfo := content.Info{
					Digest: states[i].layer.Blob.Digest,
					Labels: map[string]string{
						"containerd.io/uncompressed": states[i].layer.Diff.Digest.String(),
					},
				}
				if _, err := cs.Update(ctx, cinfo, "labels.containerd.io/uncompressed"); err != nil {
					return err
				}
			}

			chain = append(chain, states[i].layer.Diff.Digest)
			states[i].unpacked = true
			log.G(ctx).WithFields(logrus.Fields{
				"desc": states[i].layer.Blob,
				"diff": states[i].layer.Diff,
			}).Debug("layer unpacked")
		}
		// Check whether all layers are unpacked.
		if states[len(states)-1].unpacked {
			break
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

func (u *unpacker) handlerWrapper(uctx context.Context, unpacks *int32) (func(images.Handler) images.Handler, *errgroup.Group) {
	eg, uctx := errgroup.WithContext(uctx)
	return func(f images.Handler) images.Handler {
		var (
			lock    sync.Mutex
			layers  []ocispec.Descriptor
			schema1 bool
		)
		return images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			children, err := f.Handle(ctx, desc)
			if err != nil {
				return children, err
			}

			// `Pull` only supports one platform, so there is only
			// one manifest to handle, and manifest list can be
			// safely skipped.
			// TODO: support multi-platform unpack.
			switch mt := desc.MediaType; {
			case mt == images.MediaTypeDockerSchema1Manifest:
				lock.Lock()
				schema1 = true
				lock.Unlock()
			case mt == images.MediaTypeDockerSchema2Manifest || mt == ocispec.MediaTypeImageManifest:
				lock.Lock()
				for _, child := range children {
					if child.MediaType == images.MediaTypeDockerSchema2Config ||
						child.MediaType == ocispec.MediaTypeImageConfig {
						continue
					}
					layers = append(layers, child)
				}
				lock.Unlock()
			case mt == images.MediaTypeDockerSchema2Config || mt == ocispec.MediaTypeImageConfig:
				lock.Lock()
				l := append([]ocispec.Descriptor{}, layers...)
				lock.Unlock()
				if len(l) > 0 {
					atomic.AddInt32(unpacks, 1)
					eg.Go(func() error {
						return u.unpack(uctx, desc, l)
					})
				}
			case images.IsLayerType(mt):
				lock.Lock()
				update := !schema1
				lock.Unlock()
				if update {
					u.updateCh <- desc
				}
			}
			return children, nil
		})
	}, eg
}
