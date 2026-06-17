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

package windows

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio"
	winfs "github.com/Microsoft/go-winio/pkg/fs"
	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/pkg/ociwclayer"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/core/snapshots/storage"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.SnapshotPlugin,
		ID:   "windows",
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			ic.Meta.Platforms = []ocispec.Platform{platforms.DefaultSpec()}
			return NewWindowsSnapshotter(ic.Properties[plugins.PropertyRootDir])
		},
	})
}

const (
	// Label to specify that we should make a scratch space for a UtilityVM.
	uvmScratchLabel = "containerd.io/snapshot/io.microsoft.vm.storage.scratch"
	// Label to control a containers scratch space size (sandbox.vhdx).
	//
	// Deprecated: use rootfsSizeInBytesLabel
	rootfsSizeInGBLabel = "containerd.io/snapshot/io.microsoft.container.storage.rootfs.size-gb"
	// rootfsSizeInBytesLabel is a label to control a Windows containers scratch space
	// size in bytes.
	rootfsSizeInBytesLabel = "containerd.io/snapshot/windows/rootfs.sizebytes"
)

// snapshotter for legacy windows layers
type wcowSnapshotter struct {
	*windowsBaseSnapshotter
}

// NewWindowsSnapshotter returns a new windows snapshotter
func NewWindowsSnapshotter(root string) (snapshots.Snapshotter, error) {
	fsType, err := winfs.GetFileSystemType(root)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(fsType) != "ntfs" {
		return nil, fmt.Errorf("%s is not on an NTFS volume - only NTFS volumes are supported: %w", root, errdefs.ErrInvalidArgument)
	}

	baseSn, err := newBaseSnapshotter(root)
	if err != nil {
		return nil, err
	}

	return &wcowSnapshotter{
		windowsBaseSnapshotter: baseSn,
	}, nil
}

func (s *wcowSnapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindActive, key, parent, opts)
}

func (s *wcowSnapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindView, key, parent, opts)
}

func (s *wcowSnapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) (retErr error) {
	return s.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		// grab the existing id
		id, _, _, err := storage.GetInfo(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get storage info for %s: %w", key, err)
		}

		snapshot, err := storage.GetSnapshot(ctx, key)
		if err != nil {
			return err
		}

		path := s.getSnapshotDir(id)

		// If (windowsDiff).Apply was used to populate this layer, then it's already in the 'committed' state.
		// See createSnapshot below for more details
		if !strings.Contains(key, snapshots.UnpackKeyPrefix) {
			if len(snapshot.ParentIDs) == 0 {
				if err = hcsshim.ConvertToBaseLayer(path); err != nil {
					return err
				}
			} else if err := s.convertScratchToReadOnlyLayer(ctx, snapshot, path); err != nil {
				return err
			}
		}

		usage, err := fs.DiskUsage(ctx, path)
		if err != nil {
			return fmt.Errorf("failed to collect disk usage of snapshot storage: %s: %w", path, err)
		}

		if _, err := storage.CommitActive(ctx, key, name, snapshots.Usage(usage), opts...); err != nil {
			return fmt.Errorf("failed to commit snapshot: %w", err)
		}

		return nil
	})
}

func (s *wcowSnapshotter) createSnapshot(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) (_ []mount.Mount, err error) {
	var newSnapshot storage.Snapshot
	err = s.ms.WithTransaction(ctx, true, func(ctx context.Context) (retErr error) {
		newSnapshot, err = storage.CreateSnapshot(ctx, kind, key, parent, opts...)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		log.G(ctx).Debug("createSnapshot")
		// Create the new snapshot dir
		snDir := s.getSnapshotDir(newSnapshot.ID)
		if err = os.MkdirAll(snDir, 0700); err != nil {
			return fmt.Errorf("failed to create snapshot dir %s: %w", snDir, err)
		}
		defer func() {
			if retErr != nil {
				os.RemoveAll(snDir)
			}
		}()

		if strings.Contains(key, snapshots.UnpackKeyPrefix) {
			// IO/disk space optimization: Do nothing
			//
			// We only need one sandbox.vhdx for the container. Skip making one for this
			// snapshot if this isn't the snapshot that just houses the final sandbox.vhd
			// that will be mounted as the containers scratch. Currently the key for a snapshot
			// where a layer will be extracted to will have the string `extract-` in it.
			return nil
		}

		if len(newSnapshot.ParentIDs) == 0 {
			// A parentless snapshot a new base layer. Valid base layers must have a "Files" folder.
			// When committed, there'll be some post-processing to fill in the rest
			// of the metadata.
			filesDir := filepath.Join(snDir, "Files")
			if err := os.MkdirAll(filesDir, 0700); err != nil {
				return fmt.Errorf("creating Files dir: %w", err)
			}
			return nil
		}

		parentLayerPaths := s.parentIDsToParentPaths(newSnapshot.ParentIDs)
		var snapshotInfo snapshots.Info
		for _, o := range opts {
			o(&snapshotInfo)
		}

		sizeInBytes, err := getRequestedScratchSize(ctx, snapshotInfo)
		if err != nil {
			return err
		}

		var makeUVMScratch bool
		if _, ok := snapshotInfo.Labels[uvmScratchLabel]; ok {
			makeUVMScratch = true
		}

		// This has    to be run first to avoid clashing with the containers sandbox.vhdx.
		if makeUVMScratch {
			if err = s.createUVMScratchLayer(ctx, snDir, parentLayerPaths); err != nil {
				return fmt.Errorf("failed to make UVM's scratch layer: %w", err)
			}
		}
		if err = s.createScratchLayer(ctx, snDir, parentLayerPaths, sizeInBytes); err != nil {
			return fmt.Errorf("failed to create scratch layer: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.mounts(newSnapshot, key), nil
}

// Remove abandons the transaction identified by key. All resources
// associated with the key will be removed.
func (s *wcowSnapshotter) Remove(ctx context.Context, key string) error {
	renamedID, err := s.preRemove(ctx, key)
	if err != nil {
		// wrap as ErrFailedPrecondition so that cleanup of other snapshots can continue
		return fmt.Errorf("%w: %s", errdefs.ErrFailedPrecondition, err)
	}

	if err = hcsshim.DestroyLayer(s.info, renamedID); err != nil {
		// Must be cleaned up, any "rm-*" could be removed if no active transactions
		log.G(ctx).WithError(err).WithField("renamedID", renamedID).Warnf("Failed to remove root filesystem")
	}

	return nil
}

// Mounts returns the mounts for the transaction identified by key. Can be
// called on an read-write or readonly transaction.
//
// This can be used to recover mounts after calling View or Prepare.
func (s *wcowSnapshotter) Mounts(ctx context.Context, key string) (_ []mount.Mount, err error) {
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

	return s.mounts(snapshot, key), nil
}

// This is essentially a recreation of what HCS' CreateSandboxLayer does with some extra bells and
// whistles like expanding the volume if a size is specified.
func (s *wcowSnapshotter) createScratchLayer(ctx context.Context, snDir string, parentLayers []string, sizeInBytes uint64) error {
	parentLen := len(parentLayers)
	if parentLen == 0 {
		return fmt.Errorf("no parent layers present")
	}

	baseLayer := parentLayers[parentLen-1]
	templateDiffDisk := filepath.Join(baseLayer, "blank.vhdx")
	dest := filepath.Join(snDir, "sandbox.vhdx")
	if err := copyScratchDisk(templateDiffDisk, dest); err != nil {
		return err
	}

	if sizeInBytes != 0 {
		if err := hcsshim.ExpandSandboxSize(s.info, filepath.Base(snDir), sizeInBytes); err != nil {
			return fmt.Errorf("failed to expand sandbox vhdx size to %d bytes: %w", sizeInBytes, err)
		}
	}
	return nil
}

// convertScratchToReadOnlyLayer reimports the layer over itself, to transfer the files from the sandbox.vhdx to the on-disk storage.
func (s *wcowSnapshotter) convertScratchToReadOnlyLayer(ctx context.Context, snapshot storage.Snapshot, path string) (retErr error) {

	// TODO darrenstahlmsft: When this is done isolated, we should disable these.
	// it currently cannot be disabled, unless we add ref counting. Since this is
	// temporary, leaving it enabled is OK for now.
	// https://github.com/containerd/containerd/issues/1681
	if err := winio.EnableProcessPrivileges([]string{winio.SeBackupPrivilege, winio.SeRestorePrivilege}); err != nil {
		return fmt.Errorf("failed to enable necessary privileges: %w", err)
	}

	parentLayerPaths := s.parentIDsToParentPaths(snapshot.ParentIDs)
	reader, writer := io.Pipe()

	go func() {
		err := ociwclayer.ExportLayerToTar(ctx, writer, path, parentLayerPaths)
		writer.CloseWithError(err)
	}()

	// It seems that in certain situations, like having the containerd root and state on a file system hosted on a
	// mounted VHDX, we need SeSecurityPrivilege when opening a file with winio.ACCESS_SYSTEM_SECURITY. This happens
	// in the base layer writer in hcsshim when adding a new file.
	if err := winio.RunWithPrivileges([]string{winio.SeSecurityPrivilege}, func() error {
		_, err := ociwclayer.ImportLayerFromTar(ctx, reader, path, parentLayerPaths)
		return err
	}); err != nil {
		return fmt.Errorf("failed to reimport snapshot: %w", err)
	}

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("failed discarding extra data in import stream: %w", err)
	}

	// NOTE: We do not delete the sandbox.vhdx here, as that will break later calls to
	// ociwclayer.ExportLayerToTar for this snapshot.
	// As a consequence, the data for this layer is held twice, once on-disk and once
	// in the sandbox.vhdx.
	// TODO: This is either a bug or misfeature in hcsshim, so will need to be resolved
	// there first.

	return nil
}

func (s *wcowSnapshotter) mounts(sn storage.Snapshot, key string) []mount.Mount {
	var (
		roFlag string
	)

	if sn.Kind == snapshots.KindView {
		roFlag = "ro"
	} else {
		roFlag = "rw"
	}

	source := s.getSnapshotDir(sn.ID)
	parentLayerPaths := s.parentIDsToParentPaths(sn.ParentIDs)

	mountType := "windows-layer"

	// error is not checked here, as a string array will never fail to Marshal
	parentLayersJSON, _ := json.Marshal(parentLayerPaths)
	parentLayersOption := mount.ParentLayerPathsFlag + string(parentLayersJSON)

	options := []string{
		roFlag,
	}
	if len(sn.ParentIDs) != 0 {
		options = append(options, parentLayersOption)
	}
	mounts := []mount.Mount{
		{
			Source:  source,
			Type:    mountType,
			Options: options,
		},
	}

	return mounts
}
