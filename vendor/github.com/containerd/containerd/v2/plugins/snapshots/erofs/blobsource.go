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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containerd/errdefs"

	"github.com/containerd/containerd/v2/core/snapshots"
)

// labelPrefix namespaces the labels the snapshotter records about a snapshot
// for its own use.
//
// The prefix sits outside "containerd.io/snapshot/", whose labels are inherited
// from image annotations (see snapshots.FilterInheritedLabels): a manifest could
// otherwise claim its layer lives at any path on the host. The same filter runs
// on everything the metadata snapshotter passes down. A client cannot set or
// clear these labels either. They stay readable: Stat and Walk report the
// backend's labels alongside the metadata store's.
const labelPrefix = "io.containerd.erofs.v1/"

const (
	// blobSourceKindLabel records where a snapshot's layer blob comes from.
	blobSourceKindLabel = labelPrefix + "blob.source"
	// blobSourceRefLabel records which blob, in the form the kind defines. A
	// cache ref is an absolute path.
	blobSourceRefLabel = labelPrefix + "blob.ref"
)

// blobSourceKind identifies where a snapshot's layer blob comes from, which
// determines what may be done with it. How a layer is composed into a mount does
// not depend on its kind.
type blobSourceKind string

const (
	// blobSourceLocal is a blob stored in the snapshot directory itself,
	// written there by a differ. A snapshot that records no source has one.
	blobSourceLocal blobSourceKind = ""

	// blobSourceCache is a blob in an operator-owned layer content cache,
	// shared with every other snapshot of that layer. It is pre-converted, so
	// it is complete before anything is applied. It must never be written to.
	blobSourceCache blobSourceKind = "cache"
)

// blobSource records where a snapshot's layer blob comes from. Ref is
// kind-specific and empty for a local blob; for a cached one it is the absolute
// path of the cache entry, which is mounted directly.
type blobSource struct {
	Kind blobSourceKind
	Ref  string
}

// populated reports whether the blob is already complete when the snapshot is
// prepared. Nothing is fetched or applied into such a snapshot. Committing it
// does not convert anything.
func (b blobSource) populated() bool {
	return b.Kind != blobSourceLocal
}

// owned reports whether the blob belongs to this snapshot alone. Such a blob may
// be written, have its attributes changed, and be removed with the snapshot. A
// blob from any other source is shared and must be left untouched.
func (b blobSource) owned() bool {
	return b.Kind == blobSourceLocal
}

// labels returns the labels recording this source. A local blob does not record
// one. It returns nil for such a blob.
func (b blobSource) labels() map[string]string {
	if !b.populated() {
		return nil
	}
	return map[string]string{
		blobSourceKindLabel: string(b.Kind),
		blobSourceRefLabel:  b.Ref,
	}
}

// blobSourceFromInfo returns the blob source recorded on a snapshot, or a local
// one if the snapshot does not record a source.
func blobSourceFromInfo(info snapshots.Info) (blobSource, error) {
	src := blobSource{
		Kind: blobSourceKind(info.Labels[blobSourceKindLabel]),
		Ref:  info.Labels[blobSourceRefLabel],
	}
	switch src.Kind {
	case blobSourceLocal, blobSourceCache:
		// A local blob does not name a ref. Every other kind names one.
		if (src.Ref == "") != (src.Kind == blobSourceLocal) {
			return blobSource{}, fmt.Errorf("snapshot %q records a %q layer blob source with ref %q", info.Name, src.Kind, src.Ref)
		}
		if src.Kind == blobSourceLocal {
			return blobSource{}, nil
		}
		return src, nil
	default:
		// A newer version of the snapshotter recorded a source this one does
		// not implement. Mounting the blob would apply rules this version does
		// not have.
		return blobSource{}, fmt.Errorf("snapshot %q records an unknown layer blob source %q", info.Name, src.Kind)
	}
}

// privateLabels returns the subset of labels the snapshotter records for
// itself.
func privateLabels(labels map[string]string) map[string]string {
	var private map[string]string
	for k, v := range labels {
		if strings.HasPrefix(k, labelPrefix) {
			if private == nil {
				private = make(map[string]string, 2)
			}
			private[k] = v
		}
	}
	return private
}

// errNoLayerBlob is returned by resolveBlob for a snapshot that does not hold a
// layer blob at all, the normal state of an active snapshot before a differ has
// applied anything into it. A blob that is recorded but unusable returns a
// different error. errNoLayerBlob wraps os.ErrNotExist for a caller that only
// needs to know the blob is absent.
var errNoLayerBlob = fmt.Errorf("no erofs layer blob: %w", os.ErrNotExist)

// resolveBlob returns the path of a snapshot's layer blob together with where
// it came from. A local blob lives in the snapshot directory. Any other is
// mounted from its source. Nothing in the snapshot aliases content it does not
// own.
//
// A recorded blob that does not resolve is an error, not a miss. Reporting it
// as absent would let the caller treat the layer as unapplied. Commit would
// then convert into a path that does not exist and commit an empty layer.
func (s *snapshotter) resolveBlob(id string, info snapshots.Info) (string, blobSource, error) {
	src, err := blobSourceFromInfo(info)
	if err != nil {
		return "", blobSource{}, err
	}
	if src.populated() {
		// The ref names content this snapshot does not own. Stat resolves it as
		// the source publishes it, links included. A ref that names a directory
		// would otherwise pass as a complete layer: Commit measures it, records
		// the snapshot, and the mount fails later when it turns out not to be a
		// layer image.
		fi, err := os.Stat(src.Ref)
		if err != nil {
			return "", src, fmt.Errorf("layer blob %q from the %s is unusable: %w", src.Ref, src.Kind, err)
		}
		if !fi.Mode().IsRegular() {
			return "", src, fmt.Errorf("layer blob %q from the %s is not a regular file: %w", src.Ref, src.Kind, errdefs.ErrFailedPrecondition)
		}
		return src.Ref, src, nil
	}

	// A local blob is a regular file this snapshotter wrote into the snapshot
	// directory. Lstat answers without following a link. A link would hand back
	// a path outside the directory as though the snapshot owned it. Conversion
	// at Commit, fsverity, and IMMUTABLE_FL would then act on its target.
	layerBlob := s.layerBlobPath(id)
	fi, err := os.Lstat(layerBlob)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", src, fmt.Errorf("%s: %w", layerBlob, errNoLayerBlob)
		}
		return "", src, fmt.Errorf("failed to stat layer blob %q: %w", layerBlob, err)
	}
	if !fi.Mode().IsRegular() {
		return "", src, fmt.Errorf("layer blob %q is not a regular file: %w", layerBlob, errdefs.ErrFailedPrecondition)
	}
	return layerBlob, src, nil
}
