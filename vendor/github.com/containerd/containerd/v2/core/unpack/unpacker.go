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

package unpack

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/diff"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/internal/cleanup"
	"github.com/containerd/containerd/v2/internal/kmutex"
	"github.com/containerd/containerd/v2/pkg/labels"
	"github.com/containerd/containerd/v2/pkg/tracing"
)

const (
	labelSnapshotRef = "containerd.io/snapshot.ref"
	unpackSpanPrefix = "pkg.unpack.unpacker"
)

// Result returns information about the unpacks which were completed.
type Result struct {
	Unpacks int
}

type unpackerConfig struct {
	platforms []*Platform

	content content.Store

	limiter               *semaphore.Weighted
	duplicationSuppressor kmutex.KeyedLocker
}

// Platform represents a platform-specific unpack configuration which includes
// the platform matcher as well as snapshotter and applier.
type Platform struct {
	Platform platforms.Matcher

	SnapshotterKey     string
	Snapshotter        snapshots.Snapshotter
	SnapshotOpts       []snapshots.Opt
	SnapshotterExports map[string]string

	Applier   diff.Applier
	ApplyOpts []diff.ApplyOpt

	// ConfigType is the supported config type to be considered for unpacking
	// Defaults to OCI image config
	ConfigType string

	// LayerTypes are the supported types to be considered layers
	// Defaults to OCI image layers
	LayerTypes []string
}

type UnpackerOpt func(*unpackerConfig) error

func WithUnpackPlatform(u Platform) UnpackerOpt {
	return UnpackerOpt(func(c *unpackerConfig) error {
		if u.Platform == nil {
			u.Platform = platforms.All
		}
		if u.Snapshotter == nil {
			return fmt.Errorf("snapshotter must be provided to unpack")
		}
		if u.SnapshotterKey == "" {
			if s, ok := u.Snapshotter.(fmt.Stringer); ok {
				u.SnapshotterKey = s.String()
			} else {
				u.SnapshotterKey = "unknown"
			}
		}
		if u.Applier == nil {
			return fmt.Errorf("applier must be provided to unpack")
		}

		c.platforms = append(c.platforms, &u)

		return nil
	})
}

func WithLimiter(l *semaphore.Weighted) UnpackerOpt {
	return UnpackerOpt(func(c *unpackerConfig) error {
		c.limiter = l
		return nil
	})
}

func WithDuplicationSuppressor(d kmutex.KeyedLocker) UnpackerOpt {
	return UnpackerOpt(func(c *unpackerConfig) error {
		c.duplicationSuppressor = d
		return nil
	})
}

// Unpacker unpacks images by hooking into the image handler process.
// Unpacks happen in the backgrounds and waited on to complete.
type Unpacker struct {
	unpackerConfig

	unpacks atomic.Int32
	ctx     context.Context
	eg      *errgroup.Group
}

// NewUnpacker creates a new instance of the unpacker which can be used to wrap an
// image handler and unpack in parallel to handling. The unpacker will handle
// calling the block handlers when they are needed by the unpack process.
func NewUnpacker(ctx context.Context, cs content.Store, opts ...UnpackerOpt) (*Unpacker, error) {
	eg, ctx := errgroup.WithContext(ctx)

	u := &Unpacker{
		unpackerConfig: unpackerConfig{
			content:               cs,
			duplicationSuppressor: kmutex.NewNoop(),
		},
		ctx: ctx,
		eg:  eg,
	}
	for _, opt := range opts {
		if err := opt(&u.unpackerConfig); err != nil {
			return nil, err
		}
	}
	if len(u.platforms) == 0 {
		return nil, fmt.Errorf("no unpack platforms defined: %w", errdefs.ErrInvalidArgument)
	}
	return u, nil
}

// Unpack wraps an image handler to filter out blob handling and scheduling them
// during the unpack process. When an image config is encountered, the unpack
// process will be started in a goroutine.
func (u *Unpacker) Unpack(h images.Handler) images.Handler {
	var (
		lock   sync.Mutex
		layers = map[digest.Digest][]ocispec.Descriptor{}
	)

	var layerTypes map[string]bool
	var configTypes map[string]bool
	for _, p := range u.platforms {
		if p.ConfigType != "" {
			if configTypes == nil {
				configTypes = make(map[string]bool)
			}
			configTypes[p.ConfigType] = true
		}
		if len(p.LayerTypes) > 0 {
			if layerTypes == nil {
				layerTypes = make(map[string]bool)
			}
			for _, t := range p.LayerTypes {
				layerTypes[t] = true
			}
		}
	}

	return images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		ctx, span := tracing.StartSpan(ctx, tracing.Name(unpackSpanPrefix, "UnpackHandler"))
		defer span.End()
		span.SetAttributes(
			tracing.Attribute("descriptor.media.type", desc.MediaType),
			tracing.Attribute("descriptor.digest", desc.Digest.String()))
		unlock, err := u.lockBlobDescriptor(ctx, desc)
		if err != nil {
			return nil, err
		}
		children, err := h.Handle(ctx, desc)
		unlock()
		if err != nil {
			return children, err
		}

		if images.IsManifestType(desc.MediaType) {
			var nonLayers []ocispec.Descriptor
			var manifestLayers []ocispec.Descriptor
			// Split layers from non-layers, layers will be handled after
			// the config
			for i, child := range children {
				span.SetAttributes(
					tracing.Attribute("descriptor.child."+strconv.Itoa(i), []string{child.MediaType, child.Digest.String()}),
				)
				if images.IsLayerType(child.MediaType) || layerTypes[child.MediaType] {
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
		} else if images.IsConfigType(desc.MediaType) || configTypes[desc.MediaType] {
			lock.Lock()
			l := layers[desc.Digest]
			lock.Unlock()
			if len(l) > 0 {
				u.eg.Go(func() error {
					return u.unpack(h, desc, l)
				})
			}
		}
		return children, nil
	})
}

// Wait waits for any ongoing unpack processes to complete then will return
// the result.
func (u *Unpacker) Wait() (Result, error) {
	if err := u.eg.Wait(); err != nil {
		return Result{}, err
	}
	return Result{
		Unpacks: int(u.unpacks.Load()),
	}, nil
}

// unpackConfig is a subset of the OCI config for resolving rootfs and platform,
// any config type which supports the platform and rootfs field can be supported.
type unpackConfig struct {
	// Platform describes the platform which the image in the manifest runs on.
	ocispec.Platform

	// RootFS references the layer content addresses used by the image.
	RootFS ocispec.RootFS `json:"rootfs"`
}

func (u *Unpacker) unpack(
	h images.Handler,
	config ocispec.Descriptor,
	layers []ocispec.Descriptor,
) error {
	ctx := u.ctx
	ctx, layerSpan := tracing.StartSpan(ctx, tracing.Name(unpackSpanPrefix, "unpack"))
	defer layerSpan.End()
	unpackStart := time.Now()
	p, err := content.ReadBlob(ctx, u.content, config)
	if err != nil {
		return err
	}

	var i unpackConfig
	if err := json.Unmarshal(p, &i); err != nil {
		return fmt.Errorf("unmarshal image config: %w", err)
	}

	diffIDs := i.RootFS.DiffIDs
	if len(layers) != len(diffIDs) {
		return fmt.Errorf("number of layers and diffIDs don't match: %d != %d", len(layers), len(diffIDs))
	}

	// TODO: Support multiple unpacks rather than just first match
	var unpack *Platform

	imgPlatform := platforms.Normalize(i.Platform)
	for _, up := range u.platforms {
		if up.ConfigType != "" && up.ConfigType != config.MediaType {
			continue
		}
		// "layers" is only supported rootfs value for OCI images
		if (up.ConfigType == "" || images.IsConfigType(up.ConfigType)) && i.RootFS.Type != "" && i.RootFS.Type != "layers" {
			continue
		}
		if up.Platform.Match(imgPlatform) {
			unpack = up
			break
		}
	}

	if unpack == nil {
		log.G(ctx).WithField("image", config.Digest).WithField("platform", platforms.Format(imgPlatform)).Debugf("unpacker does not support platform, only fetching layers")
		return u.fetch(ctx, h, layers, nil)
	}

	u.unpacks.Add(1)

	var (
		sn = unpack.Snapshotter
		a  = unpack.Applier
		cs = u.content

		fetchOffset int
		fetchC      []chan struct{}
		fetchErr    chan error
	)

	// If there is an early return, ensure any ongoing
	// fetches get their context cancelled
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// pre-calculate chain ids for each layer
	chainIDs := make([]digest.Digest, len(diffIDs))
	copy(chainIDs, diffIDs)
	chainIDs = identity.ChainIDs(chainIDs)

	doUnpackFn := func(i int, desc ocispec.Descriptor) error {
		var parent string
		if i > 0 {
			parent = chainIDs[i-1].String()
		}
		chainID := chainIDs[i].String()

		unlock, err := u.lockSnChainID(ctx, chainID, unpack.SnapshotterKey)
		if err != nil {
			return err
		}
		defer unlock()

		// inherits annotations which are provided as snapshot labels.
		snapshotLabels := snapshots.FilterInheritedLabels(desc.Annotations)
		if snapshotLabels == nil {
			snapshotLabels = make(map[string]string)
		}
		snapshotLabels[labelSnapshotRef] = chainID

		var (
			key    string
			mounts []mount.Mount
			opts   = append(unpack.SnapshotOpts, snapshots.WithLabels(snapshotLabels))
		)

		for try := 1; try <= 3; try++ {
			// Prepare snapshot with from parent, label as root
			key = fmt.Sprintf(snapshots.UnpackKeyFormat, uniquePart(), chainID)
			mounts, err = sn.Prepare(ctx, key, parent, opts...)
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
		abort := func(ctx context.Context) {
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
			cleanup.Do(ctx, abort)
			return ctx.Err()
		case err := <-fetchErr:
			if err != nil {
				cleanup.Do(ctx, abort)
				return err
			}
		case <-fetchC[i-fetchOffset]:
		}

		diff, err := a.Apply(ctx, desc, mounts, unpack.ApplyOpts...)
		if err != nil {
			cleanup.Do(ctx, abort)
			return fmt.Errorf("failed to extract layer %s: %w", diffIDs[i], err)
		}
		if diff.Digest != diffIDs[i] {
			cleanup.Do(ctx, abort)
			return fmt.Errorf("wrong diff id calculated on extraction %q", diffIDs[i])
		}

		if err = sn.Commit(ctx, chainID, key, opts...); err != nil {
			cleanup.Do(ctx, abort)
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
				labels.LabelUncompressed: diff.Digest.String(),
			},
		}
		if _, err := cs.Update(ctx, cinfo, "labels."+labels.LabelUncompressed); err != nil {
			return err
		}
		return nil
	}

	for i, desc := range layers {
		_, layerSpan := tracing.StartSpan(ctx, tracing.Name(unpackSpanPrefix, "unpackLayer"))
		unpackLayerStart := time.Now()
		layerSpan.SetAttributes(
			tracing.Attribute("layer.media.type", desc.MediaType),
			tracing.Attribute("layer.media.size", desc.Size),
			tracing.Attribute("layer.media.digest", desc.Digest.String()),
		)
		if err := doUnpackFn(i, desc); err != nil {
			layerSpan.SetStatus(err)
			layerSpan.End()
			return err
		}
		layerSpan.End()
		log.G(ctx).WithFields(log.Fields{
			"layer":    desc.Digest,
			"duration": time.Since(unpackLayerStart),
		}).Debug("layer unpacked")
	}

	var chainID string
	if len(chainIDs) > 0 {
		chainID = chainIDs[len(chainIDs)-1].String()
	}
	cinfo := content.Info{
		Digest: config.Digest,
		Labels: map[string]string{
			fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", unpack.SnapshotterKey): chainID,
		},
	}
	_, err = cs.Update(ctx, cinfo, fmt.Sprintf("labels.containerd.io/gc.ref.snapshot.%s", unpack.SnapshotterKey))
	if err != nil {
		return err
	}
	log.G(ctx).WithFields(log.Fields{
		"config":   config.Digest,
		"chainID":  chainID,
		"duration": time.Since(unpackStart),
	}).Debug("image unpacked")

	return nil
}

func (u *Unpacker) fetch(ctx context.Context, h images.Handler, layers []ocispec.Descriptor, done []chan struct{}) error {
	eg, ctx2 := errgroup.WithContext(ctx)
	for i, desc := range layers {
		ctx2, layerSpan := tracing.StartSpan(ctx2, tracing.Name(unpackSpanPrefix, "fetchLayer"))
		layerSpan.SetAttributes(
			tracing.Attribute("layer.media.type", desc.MediaType),
			tracing.Attribute("layer.media.size", desc.Size),
			tracing.Attribute("layer.media.digest", desc.Digest.String()),
		)
		var ch chan struct{}
		if done != nil {
			ch = done[i]
		}

		if err := u.acquire(ctx); err != nil {
			return err
		}

		eg.Go(func() error {
			defer layerSpan.End()

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
			if ch != nil {
				close(ch)
			}

			return nil
		})
	}

	return eg.Wait()
}

func (u *Unpacker) acquire(ctx context.Context) error {
	if u.limiter == nil {
		return nil
	}
	return u.limiter.Acquire(ctx, 1)
}

func (u *Unpacker) release() {
	if u.limiter == nil {
		return
	}
	u.limiter.Release(1)
}

func (u *Unpacker) lockSnChainID(ctx context.Context, chainID, snapshotter string) (func(), error) {
	key := u.makeChainIDKeyWithSnapshotter(chainID, snapshotter)

	if err := u.duplicationSuppressor.Lock(ctx, key); err != nil {
		return nil, err
	}
	return func() {
		u.duplicationSuppressor.Unlock(key)
	}, nil
}

func (u *Unpacker) lockBlobDescriptor(ctx context.Context, desc ocispec.Descriptor) (func(), error) {
	key := u.makeBlobDescriptorKey(desc)

	if err := u.duplicationSuppressor.Lock(ctx, key); err != nil {
		return nil, err
	}
	return func() {
		u.duplicationSuppressor.Unlock(key)
	}, nil
}

func (u *Unpacker) makeChainIDKeyWithSnapshotter(chainID, snapshotter string) string {
	return fmt.Sprintf("sn://%s/%v", snapshotter, chainID)
}

func (u *Unpacker) makeBlobDescriptorKey(desc ocispec.Descriptor) string {
	return fmt.Sprintf("blob://%v", desc.Digest)
}

func uniquePart() string {
	t := time.Now()
	var b [3]byte
	// Ignore read failures, just decreases uniqueness
	rand.Read(b[:])
	return fmt.Sprintf("%d-%s", t.Nanosecond(), base64.URLEncoding.EncodeToString(b[:]))
}
