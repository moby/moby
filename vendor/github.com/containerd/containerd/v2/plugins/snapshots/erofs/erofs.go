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
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/containerd/continuity/fs"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/opencontainers/go-digest"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/core/snapshots/storage"
	"github.com/containerd/containerd/v2/internal/dmverity"
	"github.com/containerd/containerd/v2/internal/erofsutils"
	"github.com/containerd/containerd/v2/internal/fsverity"
	"github.com/containerd/containerd/v2/internal/userns"
)

const snapshotTempDirPrefix = "new-"

// SnapshotterConfig is used to configure the erofs snapshotter instance
type SnapshotterConfig struct {
	// ovlOptions are the base options added to the overlayfs mount (defaults to [""])
	ovlOptions []string
	// enableFsverity enables fsverity for EROFS layers
	enableFsverity bool
	// setImmutable enables IMMUTABLE_FL file attribute for EROFS layers
	setImmutable bool
	// defaultSize creates a default size writable layer for active snapshots
	defaultSize int64
	remapIDs    bool
	// dmverityMode controls dm-verity behavior: "auto" (use if .dmverity exists), "on" (require .dmverity), "off" (disable)
	dmverityMode string
	// layerContentCaches lists directories of pre-converted, diffID-keyed erofs
	// layer blobs. Each is checked one by one; the first hit is recorded as the
	// snapshot's blob source and mounted from the cache. A directory that
	// doesn't exist is treated as a cache miss. Layers missing from all of them
	// are converted normally.
	layerContentCaches []string
}

// Opt is an option to configure the erofs snapshotter
type Opt func(config *SnapshotterConfig)

// WithOvlOptions defines the extra mount options for overlayfs
func WithOvlOptions(options []string) Opt {
	return func(config *SnapshotterConfig) {
		config.ovlOptions = options
	}
}

// WithFsverity enables fsverity for EROFS layers
func WithFsverity() Opt {
	return func(config *SnapshotterConfig) {
		config.enableFsverity = true
	}
}

// WithImmutable enables IMMUTABLE_FL file attribute for EROFS layers
func WithImmutable() Opt {
	return func(config *SnapshotterConfig) {
		config.setImmutable = true
	}
}

// WithDmverityMode sets the dm-verity mode: "auto" (default), "on" (required), or "off" (disabled)
func WithDmverityMode(mode string) Opt {
	return func(config *SnapshotterConfig) {
		config.dmverityMode = mode
	}
}

// WithDefaultSize creates a default size writable layer for active snapshots
func WithDefaultSize(size int64) Opt {
	return func(config *SnapshotterConfig) {
		config.defaultSize = size
	}
}

// WithRemapIDs enables kernel ID-mapped mounts for user namespace support
func WithRemapIDs() Opt {
	return func(config *SnapshotterConfig) {
		config.remapIDs = true
	}
}

// WithLayerContentCaches configures read-only directories of pre-converted,
// diffID-keyed erofs layer blobs that the snapshotter sources layers from on
// pull instead of downloading and converting them. See the layerContentCaches
// field for details.
func WithLayerContentCaches(paths ...string) Opt {
	return func(config *SnapshotterConfig) {
		config.layerContentCaches = paths
	}
}

type MetaStore interface {
	TransactionContext(ctx context.Context, writable bool) (context.Context, storage.Transactor, error)
	WithTransaction(ctx context.Context, writable bool, fn storage.TransactionCallback) error
	Close() error
}

type snapshotter struct {
	root               string
	ms                 MetaStore
	ovlOptions         []string
	enableFsverity     bool
	setImmutable       bool
	defaultWritable    int64
	blockMode          bool
	remapIDs           bool
	dmverityMode       string
	layerContentCaches []string
}

// NewSnapshotter returns a Snapshotter which uses EROFS+OverlayFS. The layers
// are stored under the provided root. A metadata file is stored under the root.
func NewSnapshotter(root string, opts ...Opt) (snapshots.Snapshotter, error) {
	config := SnapshotterConfig{
		defaultSize: defaultWritableSize,
	}
	for _, opt := range opts {
		opt(&config)
	}

	if config.dmverityMode == "" {
		config.dmverityMode = "auto"
	}

	if config.dmverityMode != "auto" && config.dmverityMode != "on" && config.dmverityMode != "off" {
		return nil, fmt.Errorf("invalid dmverity_mode %q: must be \"auto\", \"on\", or \"off\"", config.dmverityMode)
	}

	if config.dmverityMode == "on" {
		supported, err := dmverity.IsSupported()
		if err != nil {
			return nil, fmt.Errorf("failed to check dm-verity support: %w", err)
		}
		if !supported {
			return nil, fmt.Errorf("dmverity_mode is 'on' but dm-verity is not supported on this system")
		}
	}

	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}

	if config.defaultSize == 0 {
		// If not block mode, check root compatibility
		if err := checkCompatibility(root); err != nil {
			return nil, err
		}
	}

	// A cache hit is served straight from a shared, operator-owned blob, so
	// enabling fsverity or IMMUTABLE_FL would mutate a blob other snapshots are
	// using. Both are rejected here; dm-verity is the cache's integrity
	// mechanism.
	if len(config.layerContentCaches) > 0 {
		if config.enableFsverity {
			return nil, fmt.Errorf("enable_fsverity is incompatible with layer_content_caches; use dm-verity for cache integrity")
		}
		if config.setImmutable {
			return nil, fmt.Errorf("set_immutable is incompatible with layer_content_caches")
		}

		// A cache dir is recorded in a snapshot and mounted later. A relative
		// path would resolve against the working directory of whichever
		// process performs the mount. The check is only lexical:
		// dirs are not required to exist, as a missing one just yields a cache
		// miss and may well be provisioned after startup.
		for _, dir := range config.layerContentCaches {
			if !filepath.IsAbs(dir) {
				return nil, fmt.Errorf("layer_content_caches %q must be an absolute path", dir)
			}
		}

		log.L.WithField("dirs", config.layerContentCaches).Info("erofs layer content cache enabled")
	}

	// Check fsverity support if enabled
	if config.enableFsverity {
		// TODO: Call specific function here
		supported, err := fsverity.IsSupported(root)
		if err != nil {
			return nil, fmt.Errorf("failed to check fsverity support on %q: %w", root, err)
		}
		if !supported {
			return nil, fmt.Errorf("fsverity is not supported on the filesystem of %q", root)
		}
	}

	if config.setImmutable && runtime.GOOS != "linux" {
		return nil, fmt.Errorf("setting IMMUTABLE_FL is only supported on Linux")
	}

	ms, err := storage.NewMetaStore(filepath.Join(root, "metadata.db"))
	if err != nil {
		return nil, err
	}

	if err := os.Mkdir(filepath.Join(root, "snapshots"), 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	return &snapshotter{
		root:               root,
		ms:                 ms,
		ovlOptions:         config.ovlOptions,
		enableFsverity:     config.enableFsverity,
		setImmutable:       config.setImmutable,
		defaultWritable:    config.defaultSize,
		blockMode:          config.defaultSize > 0,
		remapIDs:           config.remapIDs,
		dmverityMode:       config.dmverityMode,
		layerContentCaches: config.layerContentCaches,
	}, nil
}

// Close closes the snapshotter
func (s *snapshotter) Close() error {
	return s.ms.Close()
}

func (s *snapshotter) upperPath(id string) string {
	return filepath.Join(s.root, "snapshots", id, "fs")
}

func (s *snapshotter) workPath(id string) string {
	return filepath.Join(s.root, "snapshots", id, "work")
}

func (s *snapshotter) writablePath(id string) string {
	return filepath.Join(s.root, "snapshots", id, "rwlayer.img")
}

// writableSize returns the size (in bytes) to allocate for this active
// snapshot's writable block image. If the caller set the
// LabelSnapshotMaxSize label, that value is used; otherwise the
// snapshotter's configured default is used. Only consulted in block mode.
func (s *snapshotter) writableSize(info snapshots.Info) int64 {
	v, ok := info.Labels[snapshots.LabelSnapshotMaxSize]
	if !ok {
		return s.defaultWritable
	}
	size, err := strconv.ParseInt(v, 10, 64)
	if err != nil || size <= 0 {
		log.L.WithField("label", snapshots.LabelSnapshotMaxSize).
			WithField("value", v).
			Warn("invalid max-size label, falling back to default")
		return s.defaultWritable
	}
	return size
}

// A committed layer blob generated by the EROFS differ
func (s *snapshotter) layerBlobPath(id string) string {
	return filepath.Join(s.root, "snapshots", id, "layer.erofs")
}

func (s *snapshotter) fsMetaPath(id string) string {
	return filepath.Join(s.root, "snapshots", id, "fsmeta.erofs")
}

// lowerPath returns the path to mount a snapshot's layer blob from, wherever it
// is stored (see resolveBlob).
func (s *snapshotter) lowerPath(id string, info snapshots.Info) (string, error) {
	path, _, err := s.resolveBlob(id, info)
	return path, err
}

// parentInfos returns the info of every ancestor of the snapshot described by
// info, nearest first, in the same order as ParentIDs on the corresponding
// storage.Snapshot. Mounts are assembled outside the metadata transaction. A
// caller reads this inside the transaction that read the snapshot.
func parentInfos(ctx context.Context, info snapshots.Info) ([]snapshots.Info, error) {
	var parents []snapshots.Info
	for name := info.Parent; name != ""; {
		_, pi, _, err := storage.GetInfo(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent snapshot %q info: %w", name, err)
		}
		parents = append(parents, pi)
		name = pi.Parent
	}
	return parents, nil
}

func (s *snapshotter) prepareDirectory(ctx context.Context, snapshotDir string, kind snapshots.Kind) (string, error) {
	td, err := os.MkdirTemp(snapshotDir, snapshotTempDirPrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	if err := os.Mkdir(filepath.Join(td, "fs"), 0755); err != nil {
		return td, err
	}
	if kind == snapshots.KindActive {
		if !s.blockMode {
			if err := os.Mkdir(filepath.Join(td, "work"), 0711); err != nil {
				return td, err
			}
		}
		// Create a special file for the EROFS differ to indicate it will be
		// prepared as an EROFS layer by the EROFS snapshotter.
		if err := os.WriteFile(filepath.Join(td, ".erofslayer"), []byte{}, 0644); err != nil {
			return td, err
		}
	}

	return td, nil
}

func (s *snapshotter) mountFsMeta(snap storage.Snapshot, parents []snapshots.Info, id int) (mount.Mount, bool, error) {
	mergedMeta := s.fsMetaPath(snap.ParentIDs[id])
	if fi, err := os.Stat(mergedMeta); err != nil || fi.Size() == 0 {
		return mount.Mount{}, false, nil
	}

	m := mount.Mount{
		Source:  mergedMeta,
		Type:    "erofs",
		Options: []string{"ro", "loop"},
	}
	for j := len(snap.ParentIDs) - 1; j >= id; j-- {
		// The flattened layers are addressed by device. Each device path has to
		// be resolved to where that layer's blob is stored.
		path, _, err := s.resolveBlob(snap.ParentIDs[j], parents[j])
		if err != nil {
			return mount.Mount{}, false, err
		}
		m.Options = append(m.Options, "device="+path)
	}
	return m, true, nil
}

// applyDmverityPolicy validates and applies dm-verity policy for a layer.
// Returns the X-containerd.dmverity mount option with the plain metadata path
// if dm-verity is applicable, or empty string if dm-verity should not be used.
// The mount handler and lower runtimes can use the path to read the .dmverity file
// and get the root hash and other metadata.
func (s *snapshotter) applyDmverityPolicy(layerBlob string) (string, error) {
	// If mode is "off", skip dm-verity entirely.
	if s.dmverityMode == "off" {
		return "", nil
	}

	metadataPath := dmverity.MetadataPath(layerBlob)
	_, metadataErr := os.Stat(metadataPath)

	// Handle stat errors: distinguish between "not found" and other errors
	// (e.g., I/O errors) to avoid silently bypassing dm-verity.
	if metadataErr != nil && !os.IsNotExist(metadataErr) {
		return "", fmt.Errorf("failed to access dm-verity metadata %s: %w", metadataPath, metadataErr)
	}
	metadataExists := metadataErr == nil

	// Validate dmverityMode policy: mode "on" requires .dmverity metadata to exist
	if s.dmverityMode == "on" && !metadataExists {
		return "", fmt.Errorf("dm-verity mode is 'on' but .dmverity metadata not found for layer %s. "+
			"This may happen if the layer was created before dm-verity was enabled. "+
			"Consider cleaning up existing snapshots and re-pulling the image, "+
			"or set dmverity_mode to 'auto' to allow layers without dm-verity metadata", layerBlob)
	}

	// If metadata exists, return the metadata path to a mount option.
	// The format is: X-containerd.dmverity=<metadata-path>
	if metadataExists {
		return fmt.Sprintf("X-containerd.dmverity=%s", metadataPath), nil
	}

	return "", nil
}

// createErofsMount creates a mount specification for an EROFS layer.
// Applies dmverityMode policy and passes it to the mount handler.
func (s *snapshotter) createErofsMount(layerBlob string) (mount.Mount, error) {
	options := []string{"ro", "loop"}

	if dmverityOpt, err := s.applyDmverityPolicy(layerBlob); err != nil {
		return mount.Mount{}, err
	} else if dmverityOpt != "" {
		options = append(options, dmverityOpt)
	}

	return mount.Mount{
		Source:  layerBlob,
		Type:    "erofs",
		Options: options,
	}, nil
}

// parentMounts returns the lower mounts for snap's parents, top to bottom. A
// merged fsmeta stands in for every layer below it. The walk stops there.
func (s *snapshotter) parentMounts(snap storage.Snapshot, parents []snapshots.Info) ([]mount.Mount, error) {
	if len(parents) != len(snap.ParentIDs) {
		return nil, fmt.Errorf("got %d parent infos for %d parents of snapshot %s: %w",
			len(parents), len(snap.ParentIDs), snap.ID, errdefs.ErrInvalidArgument)
	}
	mounts := make([]mount.Mount, 0, len(snap.ParentIDs))
	for i := range snap.ParentIDs {
		// If a merged fsmeta is valid for this layer, skip the remaining bottom layers.
		// Why? Because bottom layers have been flattened with the thin fsmeta.
		m, ok, err := s.mountFsMeta(snap, parents, i)
		if err != nil {
			return nil, err
		}
		if ok {
			return append(mounts, m), nil
		}

		layerBlob, err := s.lowerPath(snap.ParentIDs[i], parents[i])
		if err != nil {
			return nil, err
		}

		if m, err = s.createErofsMount(layerBlob); err != nil {
			return nil, fmt.Errorf("failed to create erofs mount for parent %s: %w", snap.ParentIDs[i], err)
		}

		mounts = append(mounts, m)
	}
	return mounts, nil
}

// overlayMounts stacks snap's own layer on top of the given parent lowers and
// assembles the overlay mount over them.
//
// An active snapshot with a local blob contributes a writable upper for a
// differ to apply its layer into. An active snapshot whose blob is already
// populated (see blobSource) contributes the layer itself as the top lower and
// no upperdir. A view contributes nothing. Both leave the overlay read-only. A
// caller reads a read-only overlay as a layer with nothing left to apply.
//
// A populated blob is never measured for fsverity. fsverity applies to a blob
// this snapshotter wrote. NewSnapshotter rejects fsverity together with the
// layer content cache.
func (s *snapshotter) overlayMounts(snap storage.Snapshot, info snapshots.Info, parents []mount.Mount) ([]mount.Mount, error) {
	var (
		// pre holds mounts that mount templating consumes by index. They sit
		// outside the lowerdir range. In block mode that is the writable block
		// image, referenced as "{{ mount 0 }}".
		pre []mount.Mount
		// top is this snapshot's own layer, stacked above its parents.
		top      []mount.Mount
		options  []string
		writable bool
	)

	if snap.Kind == snapshots.KindActive {
		layerBlob, src, err := s.resolveBlob(snap.ID, info)
		if err != nil && !errors.Is(err, errNoLayerBlob) {
			return nil, err
		}
		if err == nil && src.populated() {
			m, err := s.createErofsMount(layerBlob)
			if err != nil {
				return nil, fmt.Errorf("failed to create erofs mount: %w", err)
			}
			top = append(top, m)
			// Nothing in this overlay is writable. The "ro" option states
			// that. A caller does not have to infer it from the missing
			// upperdir.
			options = append(options, "ro")
		} else {
			writable = true
			if s.blockMode {
				pre = append(pre, mount.Mount{
					Source: s.writablePath(snap.ID),
					Type:   "mkfs/ext4",
					Options: []string{
						"X-containerd.mkfs.fs=ext4",
						fmt.Sprintf("X-containerd.mkfs.size=%d", s.writableSize(info)),
						// TODO: Add UUID
						"rw",
						"loop",
					},
				})
				options = append(options,
					"X-containerd.mkdir.path={{ mount 0 }}/upper:0755",
					"X-containerd.mkdir.path={{ mount 0 }}/work:0755",
					"workdir={{ mount 0 }}/work",
					"upperdir={{ mount 0 }}/upper",
				)
			} else {
				options = append(options,
					fmt.Sprintf("workdir=%s", s.workPath(snap.ID)),
					fmt.Sprintf("upperdir=%s", s.upperPath(snap.ID)),
				)
			}
		}
	}

	mounts := make([]mount.Mount, 0, len(pre)+len(top)+len(parents)+1)
	mounts = append(mounts, pre...)
	// first marks the start of the lowerdir range. A merged fsmeta ends the
	// range but never moves its start: lowers stacked above it stay in range.
	first := len(mounts)
	mounts = append(mounts, top...)
	mounts = append(mounts, parents...)

	if s.remapIDs {
		if v, ok := info.Labels[snapshots.LabelSnapshotUIDMapping]; ok {
			options = append(options, fmt.Sprintf("uidmap=%s", v))
		}
		if v, ok := info.Labels[snapshots.LabelSnapshotGIDMapping]; ok {
			options = append(options, fmt.Sprintf("gidmap=%s", v))
		}
	}

	if (len(mounts) - first) == 1 {
		// End up with one single lowerdir (e.g. fsmerge on):
		// it's unsupported by overlayfs
		if !writable {
			return append(mounts, mount.Mount{
				Type:    "format/bind",
				Source:  fmt.Sprintf("{{ mount %d }}", first),
				Options: append(options, "ro", "rbind"),
			}), nil
		}
		options = append(options, fmt.Sprintf("lowerdir={{ mount %d }}", first))
	} else {
		options = append(options, fmt.Sprintf("lowerdir={{ overlay %d %d }}", first, len(mounts)-1))
	}
	options = append(options, s.ovlOptions...)

	return append(mounts, mount.Mount{
		Type:    "format/mkdir/overlay",
		Source:  "overlay",
		Options: options,
	}), nil
}

func (s *snapshotter) mounts(snap storage.Snapshot, info snapshots.Info, parents []snapshots.Info) ([]mount.Mount, error) {
	if len(snap.ParentIDs) == 0 {
		// A recorded blob that does not resolve is an error here too (see
		// resolveBlob): falling through would hand out a writable mount for a
		// layer the snapshotter has already reported complete.
		layerBlob, src, err := s.resolveBlob(snap.ID, info)
		if err != nil && !errors.Is(err, errNoLayerBlob) {
			return nil, err
		}
		if err == nil {
			// Only a blob this snapshot owns can carry fsverity because only
			// such a blob is one this snapshotter wrote.
			if s.enableFsverity && src.owned() {
				if err := s.verifyFsverity(layerBlob); err != nil {
					return nil, err
				}
			}
			m, err := s.createErofsMount(layerBlob)
			if err != nil {
				return nil, fmt.Errorf("failed to create erofs mount: %w", err)
			}
			return []mount.Mount{m}, nil
		}
		// if we only have one layer/no parents then just return a bind mount as overlay
		// will not work
		roFlag := "rw"
		if snap.Kind == snapshots.KindView {
			roFlag = "ro"
		}
		if s.blockMode {
			return []mount.Mount{
				{
					Source: s.writablePath(snap.ID),
					Type:   "mkfs/ext4",
					Options: []string{
						"X-containerd.mkfs.fs=ext4",
						fmt.Sprintf("X-containerd.mkfs.size=%d", s.writableSize(info)),
						// TODO: Add UUID
						roFlag,
						"loop",
					},
				},
				{
					Source: "{{ mount 0 }}/upper",
					Type:   "format/mkdir/bind",
					Options: []string{
						"X-containerd.mkdir.path={{ mount 0 }}/upper:0755",
						roFlag,
						"rbind",
					},
				},
			}, nil
		}
		return []mount.Mount{
			{
				Source:  s.upperPath(snap.ID),
				Type:    "bind",
				Options: []string{roFlag, "rbind"},
			},
		}, nil
	}

	// A view of a single committed layer is that layer, read-only.
	if snap.Kind != snapshots.KindActive && len(snap.ParentIDs) == 1 {
		if len(parents) != 1 {
			return nil, fmt.Errorf("got %d parent infos for 1 parent of snapshot %s: %w",
				len(parents), snap.ID, errdefs.ErrInvalidArgument)
		}
		layerBlob, err := s.lowerPath(snap.ParentIDs[0], parents[0])
		if err != nil {
			return nil, err
		}
		m, err := s.createErofsMount(layerBlob)
		if err != nil {
			return nil, fmt.Errorf("failed to create erofs mount: %w", err)
		}
		return []mount.Mount{m}, nil
	}

	lowers, err := s.parentMounts(snap, parents)
	if err != nil {
		return nil, err
	}
	return s.overlayMounts(snap, info, lowers)
}

// createSnapshot creates an active (or view) snapshot and returns its mounts.
// On an image-layer extraction whose diffID blob is in the layer content cache,
// it records the cache entry as the snapshot's blob source and returns it as a
// read-only mount. The unpacker reads that as a staged layer, skips the layer
// conversion, and commits the snapshot normally. The parent is applied either at
// Prepare (sequential unpacking) or at Commit via WithParent (parallel
// unpacking).
func (s *snapshotter) createSnapshot(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) (_ []mount.Mount, err error) {
	var (
		snap     storage.Snapshot
		td, path string
		info     snapshots.Info
		parents  []snapshots.Info
	)

	// Any image-layer extraction can be served from the cache. s.mounts hands
	// such a snapshot out read-only and its blob stays in the cache. Nothing in
	// the snapshot can write to the shared entry.
	//
	// TODO(2.5): cached layers above the unpacker's first cache miss still have
	// their blobs downloaded. Skipping those fetches needs the unpacker rework
	// planned for 2.5.
	if kind == snapshots.KindActive {
		if cacheBlob := s.lookupCache(ctx, opts...); cacheBlob != "" {
			// Mode "on" requires a sidecar. Checking it before the snapshot is
			// recorded fails Prepare for an unusable cache entry.
			if s.dmverityMode == "on" {
				if _, err := os.Stat(dmverity.MetadataPath(cacheBlob)); err != nil {
					return nil, fmt.Errorf("dm-verity mode is 'on' but the layer content cache entry %q has no .dmverity metadata: %w", cacheBlob, err)
				}
			}
			log.G(ctx).WithFields(log.Fields{
				"key":  key,
				"blob": cacheBlob,
			}).Debug("layer content cache hit, serving the cached erofs blob")
			opts = append(opts, snapshots.WithLabels(
				blobSource{Kind: blobSourceCache, Ref: cacheBlob}.labels()))
		}
	}

	defer func() {
		if err != nil {
			if td != "" {
				if err1 := os.RemoveAll(td); err1 != nil {
					log.G(ctx).WithError(err1).Warn("failed to cleanup temp snapshot directory")
				}
			}
			if path != "" {
				if err1 := os.RemoveAll(path); err1 != nil {
					log.G(ctx).WithError(err1).WithField("path", path).Error("failed to reclaim snapshot directory, directory may need removal")
					err = fmt.Errorf("failed to remove path: %v: %w", err1, err)
				}
			}
		}
	}()

	snapshotDir := filepath.Join(s.root, "snapshots")
	td, err = s.prepareDirectory(ctx, snapshotDir, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare snapshot dir: %w", err)
	}

	if err := s.ms.WithTransaction(ctx, true, func(ctx context.Context) (err error) {
		snap, err = storage.CreateSnapshot(ctx, kind, key, parent, opts...)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		_, info, _, err = storage.GetInfo(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get snapshot info: %w", err)
		}

		// In non-block mode, set the ownership of the upperdir so that
		// user-namespace-remapped containers can write to it.
		// In block mode the upperdir lives inside the block image,
		// so host ownership is irrelevant.
		if !s.blockMode {
			var (
				mappedUID, mappedGID     = -1, -1
				uidmapLabel, gidmapLabel string
				needsRemap               = false
			)
			if v, ok := info.Labels[snapshots.LabelSnapshotUIDMapping]; ok {
				uidmapLabel = v
				needsRemap = true
			}
			if v, ok := info.Labels[snapshots.LabelSnapshotGIDMapping]; ok {
				gidmapLabel = v
				needsRemap = true
			}

			if needsRemap {
				var idMap userns.IDMap
				if err = idMap.Unmarshal(uidmapLabel, gidmapLabel); err != nil {
					return fmt.Errorf("failed to unmarshal snapshot ID mapped labels: %w", err)
				}
				root, err := idMap.RootPair()
				if err != nil {
					return fmt.Errorf("failed to find root pair: %w", err)
				}
				mappedUID, mappedGID = int(root.Uid), int(root.Gid)
			}

			// Fall back to copying ownership from parent if no ID mapping labels
			if mappedUID == -1 || mappedGID == -1 {
				if len(snap.ParentIDs) > 0 {
					uid, gid, err := getParentOwnership(s.upperPath(snap.ParentIDs[0]))
					if err != nil {
						return fmt.Errorf("failed to get parent ownership: %w", err)
					}
					mappedUID = uid
					mappedGID = gid
				}
			}

			if mappedUID != -1 && mappedGID != -1 {
				if err := os.Lchown(filepath.Join(td, "fs"), mappedUID, mappedGID); err != nil {
					return fmt.Errorf("failed to chown: %w", err)
				}
			}
		}

		path = filepath.Join(snapshotDir, snap.ID)
		if err = os.Rename(td, path); err != nil {
			return fmt.Errorf("failed to rename: %w", err)
		}
		td = ""

		// Read inside the transaction (see parentInfos).
		parents, err = parentInfos(ctx, info)
		return err
	}); err != nil {
		return nil, err
	}

	return s.mounts(snap, info, parents)
}

func (s *snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindActive, key, parent, opts)
}

// cacheID identifies a layer content cache in metrics by its position in the
// configured list and the directory's base name. The path itself varies by
// host, so the same cache wouldn't aggregate across a fleet.
func cacheID(i int, dir string) string {
	return strconv.Itoa(i) + ":" + filepath.Base(dir)
}

// lookupCache returns the absolute path of the cached erofs blob that can serve
// the layer being prepared, or "" on a miss. It gates on: at least one cache
// being configured, the Prepare being an image-layer extraction (carries the
// snapshot.ref and diff-id labels), and the diffID blob being present. Caches
// are checked one by one and the first hit wins. Every miss (cache disabled,
// non-extraction Prepare, absent or unreadable cache dir, missing entry,
// malformed labels) returns "" so pulls keep working. Any dm-verity sidecar is
// derived from the blob path (via dmverity.MetadataPath).
func (s *snapshotter) lookupCache(ctx context.Context, opts ...snapshots.Opt) string {
	if len(s.layerContentCaches) == 0 {
		return ""
	}

	var base snapshots.Info
	for _, opt := range opts {
		if err := opt(&base); err != nil {
			return ""
		}
	}

	diffIDStr := base.Labels[snapshots.LabelSnapshotDiffID]
	if base.Labels[snapshots.LabelSnapshotRef] == "" || diffIDStr == "" {
		// Not an image-layer extraction, or no diffID to key on.
		return ""
	}
	diffID, err := digest.Parse(diffIDStr)
	if err != nil {
		log.G(ctx).WithError(err).WithField("diffID", diffIDStr).
			Warn("erofs layer cache: invalid diff-id label, treating as cache miss")
		return ""
	}

	for i, dir := range s.layerContentCaches {
		blob := erofsutils.CacheBlobPath(dir, diffID)
		fi, err := os.Stat(blob)
		if err != nil {
			if !os.IsNotExist(err) {
				// An unreadable cache (a down FUSE mount, a permission change since
				// startup) shouldn't fail the pull or mask a hit in a later cache.
				cacheErrors.WithValues("unreadable", cacheID(i, dir)).Inc()
				log.G(ctx).WithError(err).WithField("blob", blob).
					Warn("erofs layer cache: failed to stat cache blob, skipping this cache")
			}
			continue
		}
		cacheLookups.WithValues("hit").Inc()
		cacheHits.WithValues(cacheID(i, dir)).Inc()
		cacheHitBytes.Inc(float64(fi.Size()))
		// The path is absolute because the configured dirs are validated as
		// absolute. It is recorded in the snapshot and mounted later. It must
		// not depend on a working directory.
		return blob
	}

	cacheLookups.WithValues("miss").Inc()

	log.G(ctx).WithField("diffID", diffID.String()).Trace("erofs layer cache miss")
	return ""
}

func (s *snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindView, key, parent, opts)
}

func (s *snapshotter) commitBlock(ctx context.Context, layerBlob string, id string) error {
	layer := s.writablePath(id)
	if _, err := os.Stat(layer); err != nil {
		if os.IsNotExist(err) {
			if cerr := convertDirToErofs(ctx, layerBlob, s.upperPath(id)); cerr != nil {
				return fmt.Errorf("failed to convert upper to erofs layer: %w", cerr)
			}
			// TODO: Cleanup method?
			return nil
		}
		return fmt.Errorf("failed to access writable layer %s: %w", layer, err)
	}
	m := mount.Mount{
		Source:  layer,
		Type:    "ext4",
		Options: []string{"ro", "loop", "noload"},
	}
	if err := m.Mount(s.upperPath(id)); err != nil {
		return fmt.Errorf("failed to mount writable layer %s: %w", layer, err)
	}
	log.G(ctx).WithField("target", s.upperPath(id)).Debug("Mounted writable layer for conversion")
	upperDir := filepath.Join(s.upperPath(id), "upper")
	if _, err := os.Stat(upperDir); os.IsNotExist(err) {
		mount.Unmount(s.upperPath(id), 0)
		// upper is empty, just convert the empty directory
		upperDir = s.upperPath(id)
	} else {
		defer func() {
			mount.Unmount(s.upperPath(id), 0)
		}()
	}
	if cerr := convertDirToErofs(ctx, layerBlob, upperDir); cerr != nil {
		return fmt.Errorf("failed to convert upper block to erofs layer: %w", cerr)
	}
	return nil
}

func (s *snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	var (
		id   string
		info snapshots.Info
	)

	// Apply the overlayfs upperdir (generated by non-EROFS differs) into a EROFS blob
	// in a read transaction first since conversion could be slow.
	err := s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		sid, si, _, err := storage.GetInfo(ctx, key)
		if err != nil {
			return err
		}
		id, info = sid, si
		return err
	})
	if err != nil {
		return err
	}

	// storage.CommitActive replaces the snapshot's labels with the ones passed
	// here. The snapshotter's own labels are added back. The committed layer
	// keeps the record of where its blob is.
	if private := privateLabels(info.Labels); len(private) > 0 {
		opts = append(opts, snapshots.WithLabels(private))
	}

	// A populated blob is committed as it is: nothing needs conversion and the
	// blob belongs to its source. resolveBlob fails for a recorded source that
	// does not resolve. An unapplied layer is never committed as a complete
	// one.
	layerBlob, src, err := s.resolveBlob(id, info)
	if err != nil {
		if !errors.Is(err, errNoLayerBlob) {
			return err
		}
		// The layer was not applied by the EROFS differ, possibly the walking
		// differ. Convert the upperdir.
		layerBlob = s.layerBlobPath(id)
		if cerr := s.commitBlock(ctx, layerBlob, id); cerr != nil {
			if errdefs.IsNotImplemented(cerr) {
				return err
			}
			return cerr
		}
	}

	// Both settings write to the blob, which is only allowed on a blob this
	// snapshot owns. A shared one would be mutated while every other snapshot
	// is using it. Neither can be configured together with the layer content
	// cache (see NewSnapshotter); this states the requirement where it
	// applies.
	if src.owned() {
		// Enable fsverity on the EROFS layer if configured
		if s.enableFsverity {
			if err := fsverity.Enable(layerBlob); err != nil {
				return fmt.Errorf("failed to enable fsverity: %w", err)
			}
		}

		// Set IMMUTABLE_FL on the EROFS layer to avoid artificial data loss
		if s.setImmutable {
			if err := setImmutable(layerBlob, true); err != nil {
				log.G(ctx).WithError(err).Warnf("failed to set IMMUTABLE_FL for %s", layerBlob)
			}
		}
	}

	// Note: dm-verity formatting is handled by the EROFS differ, not here

	return s.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		if _, err := os.Stat(layerBlob); err != nil {
			return fmt.Errorf("failed to get the converted erofs blob: %w", err)
		}

		// TODO: a blob this snapshot does not own is counted in full here, even
		// though removing the snapshot does not reclaim any of it. Usage has
		// several meanings: bytes on disk, bytes a deletion would free, and
		// attribution of shared bytes. Choosing one needs its own design.
		usage, err := fs.DiskUsage(ctx, layerBlob)
		if err != nil {
			return err
		}
		if _, err = storage.CommitActive(ctx, key, name, snapshots.Usage(usage), opts...); err != nil {
			return fmt.Errorf("failed to commit snapshot %s: %w", key, err)
		}
		return nil
	})
}

func (s *snapshotter) Mounts(ctx context.Context, key string) (_ []mount.Mount, err error) {
	var (
		snap    storage.Snapshot
		info    snapshots.Info
		parents []snapshots.Info
	)
	if err := s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		snap, err = storage.GetSnapshot(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get active mount: %w", err)
		}

		_, info, _, err = storage.GetInfo(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get snapshot info: %w", err)
		}

		// Read inside the transaction (see parentInfos).
		parents, err = parentInfos(ctx, info)
		return err
	}); err != nil {
		return nil, err
	}
	return s.mounts(snap, info, parents)
}

func (s *snapshotter) getCleanupDirectories(ctx context.Context) ([]string, error) {
	ids, err := storage.IDMap(ctx)
	if err != nil {
		return nil, err
	}

	snapshotDir := filepath.Join(s.root, "snapshots")
	fd, err := os.Open(snapshotDir)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	dirs, err := fd.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	cleanup := []string{}
	for _, d := range dirs {
		// A new-* directory may belong to a concurrent snapshot creation. It is
		// renamed to its metadata ID after the writer transaction is acquired, so
		// Remove must not treat it as an orphan in the meantime.
		if strings.HasPrefix(d, snapshotTempDirPrefix) {
			continue
		}
		if _, ok := ids[d]; ok {
			continue
		}
		cleanup = append(cleanup, filepath.Join(snapshotDir, d))
	}

	return cleanup, nil
}

// Remove abandons the snapshot identified by key. The snapshot will
// immediately become unavailable and unrecoverable. Disk space will
// be freed up on the next call to `Cleanup`.
func (s *snapshotter) Remove(ctx context.Context, key string) (err error) {
	var removals []string
	var id string
	// Remove directories after the transaction is closed, failures must not
	// return error since the transaction is committed with the removal
	// key no longer available.
	defer func() {
		if err == nil {
			if err := cleanupUpper(s.upperPath(id)); err != nil {
				log.G(ctx).WithError(err).WithField("id", id).Warnf("failed to cleanup upperdir")
			}

			// Note: dm-verity device cleanup is handled by the EROFS mount handler
			// during Deactivate/Unmount, not here in Remove()

			for _, dir := range removals {
				if err := os.RemoveAll(dir); err != nil {
					log.G(ctx).WithError(err).WithField("path", dir).Warn("failed to remove directory")
				}
			}
		}
	}()
	return s.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		id, info, _, err := storage.GetInfo(ctx, key)
		if err != nil {
			if errdefs.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to get snapshot info: %w", err)
		}

		// Only a committed snapshot has a layer blob, and only a blob in the
		// snapshot directory can have had IMMUTABLE_FL set by this snapshotter.
		// A blob from any other source stays where it is. Removing the
		// directory does not reach it.
		if info.Kind == snapshots.KindCommitted {
			// setImmutable opens the path. A link here would clear the flag on
			// its target. Lstat and a regular-file check keep this on the
			// snapshot's own blob. Anything else is skipped: os.RemoveAll
			// unlinks a link without following it.
			layerBlob := s.layerBlobPath(id)
			if fi, serr := os.Lstat(layerBlob); serr == nil && fi.Mode().IsRegular() {
				// Clear IMMUTABLE_FL before removal, since this flag avoids it.
				err = setImmutable(layerBlob, false)
				if err != nil && !errdefs.IsNotImplemented(err) {
					return fmt.Errorf("failed to clear IMMUTABLE_FL: %w", err)
				}
			}
		}
		_, _, err = storage.Remove(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to remove snapshot %s: %w", key, err)
		}

		removals, err = s.getCleanupDirectories(ctx)
		if err != nil {
			return fmt.Errorf("unable to get directories for removal: %w", err)
		}
		return nil
	})
}

func (s *snapshotter) Stat(ctx context.Context, key string) (info snapshots.Info, err error) {
	err = s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		_, info, _, err = storage.GetInfo(ctx, key)
		return err
	})
	if err != nil {
		return snapshots.Info{}, err
	}

	return info, nil
}

func (s *snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (_ snapshots.Info, err error) {
	err = s.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		// A backend snapshotter only receives inherited labels. A caller cannot
		// send the snapshotter's own labels back. An Update that
		// replaces the label set would drop them and leave a snapshot that
		// cannot find its layer blob. They are merged back in here.
		_, cur, _, err := storage.GetInfo(ctx, info.Name)
		if err != nil {
			return err
		}
		if private := privateLabels(cur.Labels); len(private) > 0 {
			if info.Labels == nil {
				info.Labels = make(map[string]string, len(private))
			}
			maps.Copy(info.Labels, private)
		}

		info, err = storage.UpdateInfo(ctx, info, fieldpaths...)
		return err
	})
	if err != nil {
		return snapshots.Info{}, err
	}

	return info, nil
}

func (s *snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	return s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		return storage.WalkInfo(ctx, fn, fs...)
	})
}

// Usage returns the resources taken by the snapshot identified by key.
//
// For active snapshots, this will scan the usage of the overlay "diff" (aka
// "upper") directory and may take some time.
//
// For committed snapshots, the value is returned from the metadata database.
func (s *snapshotter) Usage(ctx context.Context, key string) (_ snapshots.Usage, err error) {
	var (
		usage snapshots.Usage
		info  snapshots.Info
		id    string
	)
	if err := s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		id, info, usage, err = storage.GetInfo(ctx, key)
		return err
	}); err != nil {
		return usage, err
	}

	if info.Kind == snapshots.KindActive {
		upperPath := s.upperPath(id)
		du, err := fs.DiskUsage(ctx, upperPath)
		if err != nil {
			// TODO(stevvooe): Consider not reporting an error in this case.
			return snapshots.Usage{}, err
		}
		usage = snapshots.Usage(du)
	}
	return usage, nil
}

// Add a method to verify fsverity
func (s *snapshotter) verifyFsverity(path string) error {
	if !s.enableFsverity {
		return nil
	}
	enabled, err := fsverity.IsEnabled(path)
	if err != nil {
		return fmt.Errorf("failed to check fsverity status: %w", err)
	}
	if !enabled {
		return fmt.Errorf("fsverity is not enabled on %s", path)
	}
	return nil
}
