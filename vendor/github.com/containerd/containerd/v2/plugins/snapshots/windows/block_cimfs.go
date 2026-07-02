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
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/pkg/cimfs"
	cimlayer "github.com/Microsoft/hcsshim/pkg/ociwclayer/cim"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/core/snapshots/storage"
	"github.com/containerd/containerd/v2/internal/kmutex"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

const (
	// An unformatted scratch is used with ReFS. This scratch gets formatted with ReFS inside the guest before we run any
	// containers. ReFS requires the minimum size of 40GB.
	minimumUnformattedScratchSizeInBytes uint64 = 40 * 1024 * 1024 * 1024
)

// BlockCIMSnapshotterConfig contains configuration options for the block CIM snapshotter.
type BlockCIMSnapshotterConfig struct {
	// EnableLayerIntegrity enables data integrity checking for CIM layers.
	// When enabled, CIMs will be verified and sealed on close to ensure tamper-proofing.
	EnableLayerIntegrity bool `toml:"enable_layer_integrity"`
	// AppendVHDFooter controls whether VHD footers are appended to layer CIMs and merged CIMs.
	AppendVHDFooter bool `toml:"append_vhd_footer"`
	// UnformattedScratch will prepare the scratch VHDs without formatting them with
	// NTFS. This is mainly useful in case when we want to format the scratch on
	// inside the guest (hyperv isolation) before starting containers. Usually, this
	// config should be used along with the EnableLayerIntegrity config. As of now,
	// you can't run process isolated windows containers if you enable this config
	// since there is nothing on the host that will format this scratch.
	UnformattedScratch bool `toml:"unformatted_scratch"`
}

// blockCIMSnapshotter is a snapshotter that uses block CIMs for storing windows container image layers.
type blockCIMSnapshotter struct {
	*windowsBaseSnapshotter
	mergeLock kmutex.KeyedLocker
	config    *BlockCIMSnapshotterConfig
}

var _ snapshots.Snapshotter = &blockCIMSnapshotter{}

func init() {
	registry.Register(&plugin.Registration{
		Type:   plugins.SnapshotPlugin,
		ID:     "blockcim",
		Config: &BlockCIMSnapshotterConfig{},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			ic.Meta.Platforms = []ocispec.Platform{platforms.DefaultSpec()}

			config, ok := ic.Config.(*BlockCIMSnapshotterConfig)
			if !ok {
				return nil, fmt.Errorf("invalid block CIM snapshotter configuration")
			}

			log.G(ic.Context).WithField("config", config).Trace("initializing blockcim snapshotter")

			return NewBlockCIMSnapshotter(ic.Properties[plugins.PropertyRootDir], config)
		},
	})
}

func NewBlockCIMSnapshotter(root string, config *BlockCIMSnapshotterConfig) (snapshots.Snapshotter, error) {
	if !cimfs.IsBlockCimSupported() {
		return nil, fmt.Errorf("host windows version doesn't support block CIMs: %w", plugin.ErrSkipPlugin)
	}

	baseSn, err := newBaseSnapshotter(root)
	if err != nil {
		return nil, err
	}

	if config.UnformattedScratch {
		// If UnformattedScratch is enabled, we don't need to create the differing VHD as
		// the scratch isn't even formatted.  We just create 1 VHD and copy it for every
		// snapshot. In cases where snapshot creation specifies a different size of the
		// VHD, then we create a new one because we can't expand unformatted VHD.  Also
		// note that unformatted VHDs are mostly used with ReFS, and ReFS requires a
		// minimum of 40GB space, so use that as the default.
		// create this VHD with `templateVHDName` as that gets copied in snapshot creation
		err = createScratchVHD(context.Background(), filepath.Join(root, templateVHDName), WithSize(minimumUnformattedScratchSizeInBytes))
	} else {
		// If UnformattedScratch is disabled, we make the base VHD and differing VHD and
		// copy the differing VHD for every new scratch snapshot. If a different size is
		// specified, we use ExpandVHD to change the size.
		err = createDifferencingScratchVHDs(context.Background(), root)

	}
	if err != nil {
		return nil, fmt.Errorf("failed to prepare scratch VHDs: %w", err)
	}

	return &blockCIMSnapshotter{
		windowsBaseSnapshotter: baseSn,
		mergeLock:              kmutex.New(),
		config:                 config,
	}, nil
}

func (s *blockCIMSnapshotter) getSnapshotDir(id string) string {
	return filepath.Join(s.root, "snapshots", id)
}

func (s *blockCIMSnapshotter) getSingleFileCIMBlockPath(id string) string {
	return filepath.Join(s.root, "snapshots", id, layerBlockName)
}

func (s *blockCIMSnapshotter) getLayerCIMPathFromCIMBlock(cimBlockPath string) string {
	return filepath.Join(cimBlockPath, layerCIMName)
}

// context MUST be a transaction context
func (s *blockCIMSnapshotter) snapshotInfoFromID(ctx context.Context, snIDs []string) (snInfos []snapshots.Info, err error) {
	idToKey, err := storage.IDMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot ID to Key map: %w", err)
	}

	for _, sid := range snIDs {
		_, info, _, err := storage.GetInfo(ctx, idToKey[sid])
		if err != nil {
			return nil, fmt.Errorf("failed to get info for snapshot %s: %w", sid, err)
		}
		snInfos = append(snInfos, info)
	}
	return snInfos, nil
}

// returns a BlockCIM representing the snapshot
func (s *blockCIMSnapshotter) getSnapshotBlockCIM(ctx context.Context, snID string, snInfo snapshots.Info) (*cimfs.BlockCIM, error) {
	if snInfo.Kind != snapshots.KindCommitted {
		return nil, fmt.Errorf("requested BlockCIM of uncommitted snapshot `%s` ", snInfo.Name)
	}

	return &cimfs.BlockCIM{
		CimName:   layerCIMName,
		Type:      cimfs.BlockCIMTypeSingleFile,
		BlockPath: s.getSingleFileCIMBlockPath(snID),
	}, nil

}

func (s *blockCIMSnapshotter) Usage(ctx context.Context, key string) (usage snapshots.Usage, err error) {
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
		path := s.getSnapshotDir(id)
		du, err := fs.DiskUsage(ctx, path)
		if err != nil {
			return snapshots.Usage{}, err
		}
		usage = snapshots.Usage(du)
	}

	return usage, nil
}

func (s *blockCIMSnapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindActive, key, parent, opts)
}

func (s *blockCIMSnapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindView, key, parent, opts)
}

func (s *blockCIMSnapshotter) Mounts(ctx context.Context, key string) (_ []mount.Mount, err error) {
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

	return s.mounts(ctx, snapshot, key)
}

func (s *blockCIMSnapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	if !strings.Contains(key, snapshots.UnpackKeyPrefix) {
		return fmt.Errorf("committing a scratch snapshot to read-only cim layer isn't supported yet")
	}

	return s.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		usage, err := s.Usage(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get usage during commit: %w", err)
		}
		if _, err := storage.CommitActive(ctx, key, name, usage, opts...); err != nil {
			return fmt.Errorf("failed to commit snapshot: %w", err)
		}

		return nil
	})
}

// Remove abandons the transaction identified by key. All resources
// associated with the key will be removed.
func (s *blockCIMSnapshotter) Remove(ctx context.Context, key string) error {
	renamedID, err := s.preRemove(ctx, key)
	if err != nil {
		// wrap as ErrFailedPrecondition so that cleanup of other snapshots can continue
		return fmt.Errorf("%w: %s", errdefs.ErrFailedPrecondition, err)
	}

	// It is a either a scratch or layer snapshot. for scratch, the VHD must have been
	// unmounted so can be deleted and for layer CIM must have been unmounted so it
	// can be deleted.
	if err = os.RemoveAll(s.getSnapshotDir(renamedID)); err != nil {
		log.G(ctx).WithError(err).WithField("renamedID", renamedID).Warnf("failed to remove snapshot")
	}
	return nil
}

func (s *blockCIMSnapshotter) createSnapshot(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) (_ []mount.Mount, err error) {
	var newSnapshot storage.Snapshot
	err = s.ms.WithTransaction(ctx, true, func(ctx context.Context) (retErr error) {
		newSnapshot, err = storage.CreateSnapshot(ctx, kind, key, parent, opts...)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		log.G(ctx).WithFields(logrus.Fields{
			"key":    key,
			"parent": parent,
			"ID":     newSnapshot.ID,
		}).Debugf("createSnapshot")

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
			return fmt.Errorf("scratch snapshot without any parents isn't supported")
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

		// This has to be run first to avoid clashing with the containers sandbox.vhdx.
		if makeUVMScratch {
			if err = s.createUVMScratchLayer(ctx, snDir, parentLayerPaths); err != nil {
				return fmt.Errorf("failed to make UVM's scratch layer: %w", err)
			}
		}
		if err = s.createScratchLayer(ctx, snDir, sizeInBytes); err != nil {
			return fmt.Errorf("failed to create scratch layer: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if !strings.Contains(key, snapshots.UnpackKeyPrefix) && len(newSnapshot.ParentIDs) > 1 {
		// We are creating a scratch snapshot, that means we will start a
		// container with this scratch snapshot and its parent layers. So we
		// should merge the parent layer CIMs. That operation can take long time.
		// All concurrent requests to create the merge should first acquire a lock
		// for that operation. Each merge operation can be uniquely identified by
		// the last layer in that merge, which in case of the scratch snapshot is
		// a direct parent of that scratch snapshot.

		prepareMergedCIMLocked := func() error {
			// function created to limit the scope of locked code and ensure
			// that the lock is always released via defer
			s.mergeLock.Lock(ctx, parent)
			defer s.mergeLock.Unlock(parent)
			return s.prepareMergedCIM(ctx, newSnapshot.ParentIDs)
		}
		if err = prepareMergedCIMLocked(); err != nil {
			return nil, fmt.Errorf("failed to prepare merged CIM: %w", err)
		}
	}

	return s.mounts(ctx, newSnapshot, key)
}

// In case of CimFS layers, the scratch VHDs are fully empty (WCIFS layers have reparse points in scratch VHDs, hence those VHDs are unique per image),
// For regular cimfs based layers we create only one scratch VHD and then copy & expand it if required.
// For block CIM based layers, if the scratch is formatted with NTFS we can expand it,
// however, if the scratch is unformatted we have to create a new VHD with the requested
// size, ExpandSandboxSize expects formatted VHD.
func (s *blockCIMSnapshotter) createScratchLayer(ctx context.Context, snDir string, sizeInBytes uint64) error {
	if s.config.UnformattedScratch {
		if sizeInBytes == 0 {
			return copyScratchDisk(filepath.Join(s.root, templateVHDName), filepath.Join(snDir, "sandbox.vhdx"))
		} else {
			if sizeInBytes < minimumUnformattedScratchSizeInBytes {
				return fmt.Errorf("scratch size MUST be at least %d GB, requested size: %d bytes", minimumUnformattedScratchSizeInBytes/(1024*1024*1024), sizeInBytes)
			}
			if err := createScratchVHD(ctx, filepath.Join(snDir, "sandbox.vhdx"), WithSize(sizeInBytes)); err != nil {
				return fmt.Errorf("failed to create scratch VHD of requested size: %w", err)
			}
		}
	} else {
		dest := filepath.Join(snDir, "sandbox.vhdx")
		if err := copyScratchDisk(filepath.Join(s.root, templateVHDName), dest); err != nil {
			return err
		}

		if sizeInBytes != 0 {
			if err := hcsshim.ExpandSandboxSize(s.info, filepath.Base(snDir), sizeInBytes); err != nil {
				return fmt.Errorf("failed to expand sandbox vhdx size to %d bytes: %w", sizeInBytes, err)
			}
		}
	}
	return nil
}

func (s *blockCIMSnapshotter) mounts(ctx context.Context, sn storage.Snapshot, key string) ([]mount.Mount, error) {
	var (
		m            mount.Mount
		blockType    string
		parentSnInfo []snapshots.Info
		parentLayers []*cimfs.BlockCIM
	)
	m.Type = "BlockCIM"
	blockType = "file"

	err := s.ms.WithTransaction(ctx, false, func(tCtx context.Context) error {
		var tErr error
		parentSnInfo, tErr = s.snapshotInfoFromID(tCtx, sn.ParentIDs)
		if tErr != nil {
			return fmt.Errorf("failed to get info for snapshots: %w", tErr)
		}

		for i := 0; i < len(parentSnInfo); i++ {
			sCIM, tErr := s.getSnapshotBlockCIM(tCtx, sn.ParentIDs[i], parentSnInfo[i])
			if tErr != nil {
				return tErr
			}
			parentLayers = append(parentLayers, sCIM)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get parent snapshot CIMs: %w", err)
	}

	parentLayerPaths := make([]string, 0, len(parentLayers))
	for _, pl := range parentLayers {
		parentLayerPaths = append(parentLayerPaths, filepath.Join(pl.BlockPath, pl.CimName))
	}
	parentLayersJSON, _ := json.Marshal(parentLayerPaths)
	m.Options = append(m.Options, mount.ParentLayerCimPathsFlag+string(parentLayersJSON))
	m.Options = append(m.Options, mount.BlockCIMTypeFlag+blockType)

	// Add configuration options as mount options
	if s.config.EnableLayerIntegrity {
		m.Options = append(m.Options, mount.EnableLayerIntegrityFlag)
	}

	if s.config.AppendVHDFooter {
		m.Options = append(m.Options, mount.AppendVHDFooterFlag)
	}

	isScratch, err := s.isScratchSnapshot(sn.ID)
	if err != nil {
		return nil, err
	}

	if isScratch {
		if len(sn.ParentIDs) > 1 {
			mergeBlockCIMPath := filepath.Join(s.getSnapshotDir(sn.ParentIDs[0]), mergedBlockName, mergedCIMName)
			m.Options = append(m.Options, mount.MergedCIMPathFlag+mergeBlockCIMPath)
		}
		m.Source = s.getSnapshotDir(sn.ID)
	} else {
		m.Source = s.getLayerCIMPathFromCIMBlock(s.getSingleFileCIMBlockPath(sn.ID))
	}

	log.G(ctx).WithFields(logrus.Fields{
		"snapshot ID":   sn.ID,
		"snapshot name": key,
		"parent IDs":    sn.ParentIDs,
		"mount":         m,
	}).Debugf("snapshot mounts")

	return []mount.Mount{m}, nil
}

const (
	layerCIMName    = "layer.cim"
	layerBlockName  = "layer.vhd"
	mergedCIMName   = "merged.cim"
	mergedBlockName = "merged.vhd"
)

// prepareMergedCIM creates a new merged block CIM from the given snapshots. The order of
// `snapshotIDs` is important. It is expected that the snapshot ID at the last index is
// the base layer and the snapshot ID at the 0th index is the immediate parent layer.
// Newly created merged CIM is stored in the same directory as that of the snapshot at 0th
// index.  already exists in the 0th snapshot's directory, nothing is done.
func (s *blockCIMSnapshotter) prepareMergedCIM(ctx context.Context, snapshotIDs []string) (rErr error) {
	if len(snapshotIDs) < 2 {
		return fmt.Errorf("merging CIM requires at least 2 snapshots")
	}
	log.G(ctx).WithFields(logrus.Fields{
		"source snapshots": snapshotIDs,
	}).Debugf("preparing merged CIM")

	var (
		// directory in which the merged CIM is stored
		mergeDir          = s.getSnapshotDir(snapshotIDs[0])
		mergeBlockCIMPath = filepath.Join(mergeDir, mergedBlockName)
		snInfos           []snapshots.Info
	)

	// check if a merged CIM already exists
	if _, err := os.Stat(mergeBlockCIMPath); err == nil {
		log.G(ctx).Debugf("merged CIM already exists.")
		return nil
	}

	err := s.ms.WithTransaction(ctx, false, func(tctx context.Context) error {
		var terr error
		snInfos, terr = s.snapshotInfoFromID(tctx, snapshotIDs)
		if terr != nil {
			return fmt.Errorf("failed to get info for snapshots: %w", terr)
		}
		return nil
	})
	if err != nil {
		return err
	}

	sourceCIMs := make([]*cimfs.BlockCIM, 0, len(snapshotIDs))
	for i := 0; i < len(snapshotIDs); i++ {
		sCIM, err := s.getSnapshotBlockCIM(ctx, snapshotIDs[i], snInfos[i])
		if err != nil {
			return err
		}
		sourceCIMs = append(sourceCIMs, sCIM)
	}

	mergedCIM := &cimfs.BlockCIM{
		Type:      cimfs.BlockCIMTypeSingleFile,
		BlockPath: mergeBlockCIMPath,
		CimName:   "merged.cim",
	}

	// Build merge options based on snapshotter configuration
	var mergeOpts []cimlayer.BlockCIMLayerImportOpt
	if s.config.EnableLayerIntegrity {
		mergeOpts = append(mergeOpts, cimlayer.WithLayerIntegrity())
	}
	if s.config.AppendVHDFooter {
		mergeOpts = append(mergeOpts, cimlayer.WithVHDFooter())
	}

	err = cimlayer.MergeBlockCIMLayersWithOpts(ctx, sourceCIMs, mergedCIM, mergeOpts...)
	if err != nil {
		return fmt.Errorf("failed to merge CIMs: %w", err)
	}

	log.G(ctx).WithFields(logrus.Fields{
		"merged CIM": mergedCIM,
	}).Debugf("merged CIM created")

	return nil
}

func (s *blockCIMSnapshotter) isScratchSnapshot(snID string) (bool, error) {
	scratchDiskPath := filepath.Join(s.getSnapshotDir(snID), "sandbox.vhdx")
	if _, err := os.Stat(scratchDiskPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
