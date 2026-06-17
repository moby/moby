//go:build windows
// +build windows

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

package lcow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	winfs "github.com/Microsoft/go-winio/pkg/fs"
	"github.com/Microsoft/hcsshim/pkg/go-runhcs"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/core/snapshots/storage"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.SnapshotPlugin,
		ID:   "windows-lcow",
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			ic.Meta.Platforms = append(ic.Meta.Platforms, ocispec.Platform{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			})
			return NewSnapshotter(ic.Properties[plugins.PropertyRootDir])
		},
	})
}

const (
	rootfsSizeLabel           = "containerd.io/snapshot/io.microsoft.container.storage.rootfs.size-gb"
	rootfsLocLabel            = "containerd.io/snapshot/io.microsoft.container.storage.rootfs.location"
	reuseScratchLabel         = "containerd.io/snapshot/io.microsoft.container.storage.reuse-scratch"
	reuseScratchOwnerKeyLabel = "containerd.io/snapshot/io.microsoft.owner.key"
)

type snapshotter struct {
	root string
	ms   *storage.MetaStore

	scratchLock sync.Mutex
}

// NewSnapshotter returns a new windows snapshotter
func NewSnapshotter(root string) (snapshots.Snapshotter, error) {
	fsType, err := winfs.GetFileSystemType(root)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(fsType) != "ntfs" {
		return nil, fmt.Errorf("%s is not on an NTFS volume - only NTFS volumes are supported: %w", root, errdefs.ErrInvalidArgument)
	}

	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	ms, err := storage.NewMetaStore(filepath.Join(root, "metadata.db"))
	if err != nil {
		return nil, err
	}

	if err := os.Mkdir(filepath.Join(root, "snapshots"), 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	return &snapshotter{
		root: root,
		ms:   ms,
	}, nil
}

// Stat returns the info for an active or committed snapshot by name or
// key.
//
// Should be used for parent resolution, existence checks and to discern
// the kind of snapshot.
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
		info, err = storage.UpdateInfo(ctx, info, fieldpaths...)
		return err
	})
	if err != nil {
		return snapshots.Info{}, err
	}

	return info, nil
}

func (s *snapshotter) Usage(ctx context.Context, key string) (usage snapshots.Usage, err error) {
	var (
		id   string
		info snapshots.Info
	)

	err = s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		id, info, usage, err = storage.GetInfo(ctx, key)
		return err
	})
	if err != nil {
		return snapshots.Usage{}, err
	}

	if info.Kind == snapshots.KindActive {
		du, err := fs.DiskUsage(ctx, s.getSnapshotDir(id))
		if err != nil {
			return snapshots.Usage{}, err
		}
		usage = snapshots.Usage(du)
	}

	return usage, nil
}

func (s *snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindActive, key, parent, opts)
}

func (s *snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindView, key, parent, opts)
}

// Mounts returns the mounts for the transaction identified by key. Can be
// called on an read-write or readonly transaction.
//
// This can be used to recover mounts after calling View or Prepare.
func (s *snapshotter) Mounts(ctx context.Context, key string) (_ []mount.Mount, err error) {
	var snapshot storage.Snapshot
	err = s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		snapshot, err = storage.GetSnapshot(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get snapshot mount: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.mounts(snapshot), nil
}

func (s *snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	return s.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		// grab the existing id
		id, _, _, err := storage.GetInfo(ctx, key)
		if err != nil {
			return err
		}

		usage, err := fs.DiskUsage(ctx, s.getSnapshotDir(id))
		if err != nil {
			return err
		}

		if _, err = storage.CommitActive(ctx, key, name, snapshots.Usage(usage), opts...); err != nil {
			return fmt.Errorf("failed to commit snapshot: %w", err)
		}
		return nil
	})
}

// Remove abandons the transaction identified by key. All resources
// associated with the key will be removed.
func (s *snapshotter) Remove(ctx context.Context, key string) (err error) {
	var (
		renamed, path string
		restore       bool
	)

	err = s.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		id, _, err := storage.Remove(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to remove: %w", err)
		}

		path = s.getSnapshotDir(id)
		renamed = s.getSnapshotDir("rm-" + id)
		if err = os.Rename(path, renamed); err != nil && !os.IsNotExist(err) {
			return err
		}

		restore = true
		return nil
	})
	if err != nil {
		if restore { // failed to commit
			if err1 := os.Rename(renamed, path); err1 != nil {
				// May cause inconsistent data on disk
				log.G(ctx).WithError(err1).WithField("path", renamed).Error("Failed to rename after failed commit")
			}
		}
		return err
	}

	if err = os.RemoveAll(renamed); err != nil {
		// Must be cleaned up, any "rm-*" could be removed if no active transactions
		log.G(ctx).WithError(err).WithField("path", renamed).Warnf("Failed to remove root filesystem")
	}

	return nil
}

// Walk the committed snapshots.
func (s *snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	return s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		return storage.WalkInfo(ctx, fn, fs...)
	})
}

// Close closes the snapshotter
func (s *snapshotter) Close() error {
	return s.ms.Close()
}

func (s *snapshotter) mounts(sn storage.Snapshot) []mount.Mount {
	var (
		roFlag           string
		source           string
		parentLayerPaths []string
	)

	if sn.Kind == snapshots.KindView {
		roFlag = "ro"
	} else {
		roFlag = "rw"
	}

	if len(sn.ParentIDs) == 0 || sn.Kind == snapshots.KindActive {
		source = s.getSnapshotDir(sn.ID)
		parentLayerPaths = s.parentIDsToParentPaths(sn.ParentIDs)
	} else {
		source = s.getSnapshotDir(sn.ParentIDs[0])
		parentLayerPaths = s.parentIDsToParentPaths(sn.ParentIDs[1:])
	}

	// error is not checked here, as a string array will never fail to Marshal
	parentLayersJSON, _ := json.Marshal(parentLayerPaths)
	parentLayersOption := mount.ParentLayerPathsFlag + string(parentLayersJSON)

	var mounts []mount.Mount
	mounts = append(mounts, mount.Mount{
		Source: source,
		Type:   "lcow-layer",
		Options: []string{
			roFlag,
			parentLayersOption,
		},
	})

	return mounts
}

func (s *snapshotter) getSnapshotDir(id string) string {
	return filepath.Join(s.root, "snapshots", id)
}

func (s *snapshotter) createSnapshot(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) ([]mount.Mount, error) {
	var newSnapshot storage.Snapshot
	err := s.ms.WithTransaction(ctx, true, func(ctx context.Context) (err error) {
		newSnapshot, err = storage.CreateSnapshot(ctx, kind, key, parent, opts...)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		if kind != snapshots.KindActive {
			return nil
		}

		log.G(ctx).Debug("createSnapshot active")
		// Create the new snapshot dir
		snDir := s.getSnapshotDir(newSnapshot.ID)
		if err = os.MkdirAll(snDir, 0700); err != nil {
			return err
		}

		var snapshotInfo snapshots.Info
		for _, o := range opts {
			o(&snapshotInfo)
		}

		defer func() {
			if err != nil {
				os.RemoveAll(snDir)
			}
		}()

		// IO/disk space optimization
		//
		// We only need one sandbox.vhd for the container. Skip making one for this
		// snapshot if this isn't the snapshot that just houses the final sandbox.vhd
		// that will be mounted as the containers scratch. The key for a snapshot
		// where a layer.vhd will be extracted to it will have the substring `extract-` in it.
		// If this is changed this will also need to be changed.
		//
		// We save about 17MB per layer (if the default scratch vhd size of 20GB is used) and of
		// course the time to copy the vhdx per snapshot.
		if !strings.Contains(key, snapshots.UnpackKeyPrefix) {
			// This is the code path that handles re-using a scratch disk that has already been
			// made/mounted for an LCOW UVM. In the non sharing case, we create a new disk and mount this
			// into the LCOW UVM for every container but there are certain scenarios where we'd rather
			// just mount a single disk and then have every container share this one storage space instead of
			// every container having it's own xGB of space to play around with.
			//
			// This is accomplished by just making a symlink to the disk that we'd like to share and then
			// using ref counting later on down the stack in hcsshim if we see that we've already mounted this
			// disk.
			shareScratch := snapshotInfo.Labels[reuseScratchLabel]
			ownerKey := snapshotInfo.Labels[reuseScratchOwnerKeyLabel]
			if shareScratch == "true" && ownerKey != "" {
				if err = s.handleSharing(ctx, ownerKey, snDir); err != nil {
					return err
				}
			} else {
				var sizeGB int
				if sizeGBstr, ok := snapshotInfo.Labels[rootfsSizeLabel]; ok {
					i64, _ := strconv.ParseInt(sizeGBstr, 10, 32)
					sizeGB = int(i64)
				}

				scratchLocation := snapshotInfo.Labels[rootfsLocLabel]
				scratchSource, err := s.openOrCreateScratch(ctx, sizeGB, scratchLocation)
				if err != nil {
					return err
				}
				defer scratchSource.Close()

				// Create the sandbox.vhdx for this snapshot from the cache
				destPath := filepath.Join(snDir, "sandbox.vhdx")
				dest, err := os.OpenFile(destPath, os.O_RDWR|os.O_CREATE, 0700)
				if err != nil {
					return fmt.Errorf("failed to create sandbox.vhdx in snapshot: %w", err)
				}
				defer dest.Close()
				if _, err := io.Copy(dest, scratchSource); err != nil {
					dest.Close()
					os.Remove(destPath)
					return fmt.Errorf("failed to copy cached scratch.vhdx to sandbox.vhdx in snapshot: %w", err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.mounts(newSnapshot), nil
}

func (s *snapshotter) handleSharing(ctx context.Context, id, snDir string) error {
	var key string
	if err := s.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		if strings.Contains(info.Name, id) {
			key = info.Name
		}
		return nil
	}); err != nil {
		return err
	}

	mounts, err := s.Mounts(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get mounts for owner snapshot: %w", err)
	}

	sandboxPath := filepath.Join(mounts[0].Source, "sandbox.vhdx")
	linkPath := filepath.Join(snDir, "sandbox.vhdx")
	if _, err := os.Stat(sandboxPath); err != nil {
		return fmt.Errorf("failed to find sandbox.vhdx in snapshot directory: %w", err)
	}

	// We've found everything we need, now just make a symlink in our new snapshot to the
	// sandbox.vhdx in the scratch we're asking to share.
	if err := os.Symlink(sandboxPath, linkPath); err != nil {
		return fmt.Errorf("failed to create symlink for sandbox scratch space: %w", err)
	}
	return nil
}

func (s *snapshotter) openOrCreateScratch(ctx context.Context, sizeGB int, scratchLoc string) (_ *os.File, err error) {
	// Create the scratch.vhdx cache file if it doesn't already exit.
	s.scratchLock.Lock()
	defer s.scratchLock.Unlock()

	vhdFileName := "scratch.vhdx"
	if sizeGB > 0 {
		vhdFileName = fmt.Sprintf("scratch_%d.vhdx", sizeGB)
	}

	scratchFinalPath := filepath.Join(s.root, vhdFileName)
	if scratchLoc != "" {
		scratchFinalPath = filepath.Join(scratchLoc, vhdFileName)
	}

	scratchSource, err := os.OpenFile(scratchFinalPath, os.O_RDONLY, 0700)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to open vhd %s for read: %w", vhdFileName, err)
		}

		log.G(ctx).Debugf("vhdx %s not found, creating a new one", vhdFileName)

		// Golang logic for os.CreateTemp without the file creation
		r := uint32(time.Now().UnixNano() + int64(os.Getpid()))
		r = r*1664525 + 1013904223 // constants from Numerical Recipes

		scratchTempName := fmt.Sprintf("scratch-%s-tmp.vhdx", strconv.Itoa(int(1e9 + r%1e9))[1:])
		scratchTempPath := filepath.Join(s.root, scratchTempName)

		// Create the scratch
		rhcs := runhcs.Runhcs{
			Debug:     true,
			Log:       filepath.Join(s.root, "runhcs-scratch.log"),
			LogFormat: runhcs.JSON,
			Owner:     "containerd",
		}

		opt := runhcs.CreateScratchOpts{
			SizeGB: sizeGB,
		}

		if err := rhcs.CreateScratchWithOpts(ctx, scratchTempPath, &opt); err != nil {
			os.Remove(scratchTempPath)
			return nil, fmt.Errorf("failed to create '%s' temp file: %w", scratchTempName, err)
		}
		if err := os.Rename(scratchTempPath, scratchFinalPath); err != nil {
			os.Remove(scratchTempPath)
			return nil, fmt.Errorf("failed to rename '%s' temp file to 'scratch.vhdx': %w", scratchTempName, err)
		}
		scratchSource, err = os.OpenFile(scratchFinalPath, os.O_RDONLY, 0700)
		if err != nil {
			os.Remove(scratchFinalPath)
			return nil, fmt.Errorf("failed to open scratch.vhdx for read after creation: %w", err)
		}
	} else {
		log.G(ctx).Debugf("scratch vhd %s was already present. Retrieved from cache", vhdFileName)
	}
	return scratchSource, nil
}

func (s *snapshotter) parentIDsToParentPaths(parentIDs []string) []string {
	var parentLayerPaths []string
	for _, ID := range parentIDs {
		parentLayerPaths = append(parentLayerPaths, s.getSnapshotDir(ID))
	}
	return parentLayerPaths
}
