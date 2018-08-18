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

package rootfs

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"

	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Layer represents the descriptors for a layer diff. These descriptions
// include the descriptor for the uncompressed tar diff as well as a blob
// used to transport that tar. The blob descriptor may or may not describe
// a compressed object.
type Layer struct {
	Diff ocispec.Descriptor
	Blob ocispec.Descriptor
}

// ApplyLayers applies all the layers using the given snapshotter and applier.
// The returned result is a chain id digest representing all the applied layers.
// Layers are applied in order they are given, making the first layer the
// bottom-most layer in the layer chain.
func ApplyLayers(ctx context.Context, layers []Layer, sn snapshots.Snapshotter, a diff.Applier) (digest.Digest, error) {
	chain := make([]digest.Digest, len(layers))
	for i, layer := range layers {
		chain[i] = layer.Diff.Digest
	}
	chainID := identity.ChainID(chain)

	// Just stat top layer, remaining layers will have their existence checked
	// on prepare. Calling prepare on upper layers first guarantees that upper
	// layers are not removed while calling stat on lower layers
	_, err := sn.Stat(ctx, chainID.String())
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return "", errors.Wrapf(err, "failed to stat snapshot %s", chainID)
		}

		if err := applyLayers(ctx, layers, chain, sn, a); err != nil && !errdefs.IsAlreadyExists(err) {
			return "", err
		}
	}

	return chainID, nil
}

// ApplyLayer applies a single layer on top of the given provided layer chain,
// using the provided snapshotter and applier. If the layer was unpacked true
// is returned, if the layer already exists false is returned.
func ApplyLayer(ctx context.Context, layer Layer, chain []digest.Digest, sn snapshots.Snapshotter, a diff.Applier, opts ...snapshots.Opt) (bool, error) {
	var (
		chainID = identity.ChainID(append(chain, layer.Diff.Digest)).String()
		applied bool
	)
	if _, err := sn.Stat(ctx, chainID); err != nil {
		if !errdefs.IsNotFound(err) {
			return false, errors.Wrapf(err, "failed to stat snapshot %s", chainID)
		}

		if err := applyLayers(ctx, []Layer{layer}, append(chain, layer.Diff.Digest), sn, a, opts...); err != nil {
			if !errdefs.IsAlreadyExists(err) {
				return false, err
			}
		} else {
			applied = true
		}
	}
	return applied, nil
}

func applyLayers(ctx context.Context, layers []Layer, chain []digest.Digest, sn snapshots.Snapshotter, a diff.Applier, opts ...snapshots.Opt) error {
	var (
		parent  = identity.ChainID(chain[:len(chain)-1])
		chainID = identity.ChainID(chain)
		layer   = layers[len(layers)-1]
		diff    ocispec.Descriptor
		key     string
		mounts  []mount.Mount
		err     error
	)

	for {
		key = fmt.Sprintf("extract-%s %s", uniquePart(), chainID)

		// Prepare snapshot with from parent, label as root
		mounts, err = sn.Prepare(ctx, key, parent.String(), opts...)
		if err != nil {
			if errdefs.IsNotFound(err) && len(layers) > 1 {
				if err := applyLayers(ctx, layers[:len(layers)-1], chain[:len(chain)-1], sn, a); err != nil {
					if !errdefs.IsAlreadyExists(err) {
						return err
					}
				}
				// Do no try applying layers again
				layers = nil
				continue
			} else if errdefs.IsAlreadyExists(err) {
				// Try a different key
				continue
			}

			// Already exists should have the caller retry
			return errors.Wrapf(err, "failed to prepare extraction snapshot %q", key)

		}
		break
	}
	defer func() {
		if err != nil {
			if !errdefs.IsAlreadyExists(err) {
				log.G(ctx).WithError(err).WithField("key", key).Infof("apply failure, attempting cleanup")
			}

			if rerr := sn.Remove(ctx, key); rerr != nil {
				log.G(ctx).WithError(rerr).WithField("key", key).Warnf("extraction snapshot removal failed")
			}
		}
	}()

	diff, err = a.Apply(ctx, layer.Blob, mounts)
	if err != nil {
		err = errors.Wrapf(err, "failed to extract layer %s", layer.Diff.Digest)
		return err
	}
	if diff.Digest != layer.Diff.Digest {
		err = errors.Errorf("wrong diff id calculated on extraction %q", diff.Digest)
		return err
	}

	if err = sn.Commit(ctx, chainID.String(), key, opts...); err != nil {
		err = errors.Wrapf(err, "failed to commit snapshot %s", key)
		return err
	}

	return nil
}

func uniquePart() string {
	t := time.Now()
	var b [3]byte
	// Ignore read failures, just decreases uniqueness
	rand.Read(b[:])
	return fmt.Sprintf("%d-%s", t.Nanosecond(), base64.URLEncoding.EncodeToString(b[:]))
}
