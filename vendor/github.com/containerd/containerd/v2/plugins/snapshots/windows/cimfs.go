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
	"syscall"

	"github.com/Microsoft/go-winio/pkg/security"
	"github.com/Microsoft/go-winio/vhd"
	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/computestorage"
	"github.com/Microsoft/hcsshim/pkg/cimfs"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/core/snapshots/storage"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sys/windows"
)

const (
	baseVHDName                         = "blank-base.vhdx"
	templateVHDName                     = "blank.vhdx"
	defaultScratchVHDSizeInBytes uint64 = 10 * 1024 * 1024 * 1024 // 10 GB
	vhdBlockSizeInBytes          uint32 = 1 * 1024 * 1024         // 1 MB
)

// scratchCreationOpt is a functional option for configuring scratch VHD creation
type scratchCreationOpt func(*scratchCreationOptions) error

// scratchCreationOptions holds configuration for scratch VHD creation
type scratchCreationOptions struct {
	ntfsFormat  bool
	sizeInBytes uint64
}

// WithNTFSFormat specifies whether the scratch VHD should be formatted with NTFS
func WithNTFSFormat() scratchCreationOpt {
	return func(opts *scratchCreationOptions) error {
		opts.ntfsFormat = true
		return nil
	}
}

// WithSize specifies the size of the scratch VHD in bytes
func WithSize(size uint64) scratchCreationOpt {
	return func(opts *scratchCreationOptions) error {
		if size == 0 {
			return fmt.Errorf("VHD size cannot be zero")
		}
		opts.sizeInBytes = size
		return nil
	}
}

// defaultScratchCreationOptions returns the default options for scratch VHD creation
func defaultScratchCreationOptions() *scratchCreationOptions {
	return &scratchCreationOptions{
		ntfsFormat:  true,
		sizeInBytes: defaultScratchVHDSizeInBytes,
	}
}

// Composite image FileSystem (CimFS) is a new read-only filesystem (similar to overlayFS on Linux) created
// specifically for storing container image layers on windows.  cimFSSnapshotter is a snapshotter that uses
// CimFS to create read-only parent layer snapshots. Each snapshot is represented by a `<snapshot-id>.cim`
// file and some other files (region & objectid files) which hold contents of that snapshot.  Once a cim file for a layer is created it
// can only be used as a read-only layer by mounting it to a volume. Hence, CimFs will not be used when we are
// creating writable layers for container scratch and such. (However, in the future scratch layer of a container can be
// exported to a cim layer and then be used as a parent layer for another container).
type cimFSSnapshotter struct {
	*windowsBaseSnapshotter
	// cimDir is the path to the directory which holds all of the layer cim files.  CimFS needs all the
	// layer cim files to be present in the same directory. Hence, cim files of all the snapshots (even if
	// they are of different images) will be kept in the same directory.
	cimDir string
}

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.SnapshotPlugin,
		ID:   "cimfs",
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			ic.Meta.Platforms = []ocispec.Platform{platforms.DefaultSpec()}
			return NewCimFSSnapshotter(ic.Properties[plugins.PropertyRootDir])
		},
	})
}

// NewCimFSSnapshotter returns a new CimFS based windows snapshotter
func NewCimFSSnapshotter(root string) (snapshots.Snapshotter, error) {
	if !cimfs.IsCimFSSupported() {
		return nil, fmt.Errorf("host windows version doesn't support CimFS: %w", plugin.ErrSkipPlugin)
	}

	baseSn, err := newBaseSnapshotter(root)
	if err != nil {
		return nil, err
	}

	if err = createDifferencingScratchVHDs(context.Background(), baseSn.root); err != nil {
		return nil, fmt.Errorf("failed to init base scratch VHD: %w", err)
	}

	if err = os.MkdirAll(filepath.Join(baseSn.info.HomeDir, "cim-layers"), 0755); err != nil {
		return nil, err
	}

	return &cimFSSnapshotter{
		windowsBaseSnapshotter: baseSn,
		cimDir:                 filepath.Join(baseSn.info.HomeDir, "cim-layers"),
	}, nil
}

// getLayerCimPath returns the path of the cim file for the given snapshot. Note that this function doesn't
// actually check if the cim layer exists it simply does string manipulation to generate the path isCimLayer
// can be used to verify if it is actually a cim layer.
func (s *cimFSSnapshotter) getLayerCimPath(snID string) string {
	return filepath.Join(s.cimDir, (snID + ".cim"))
}

func (s *cimFSSnapshotter) parentIDsToCimPaths(parentIDs []string) []string {
	cimPaths := make([]string, 0, len(parentIDs))
	for _, id := range parentIDs {
		cimPaths = append(cimPaths, s.getLayerCimPath(id))
	}
	return cimPaths
}

func (s *cimFSSnapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	baseUsage, err := s.windowsBaseSnapshotter.Usage(ctx, key)
	if err != nil {
		return snapshots.Usage{}, err
	}

	ctx, t, err := s.ms.TransactionContext(ctx, false)
	if err != nil {
		return snapshots.Usage{}, err
	}
	defer t.Rollback()

	id, info, _, err := storage.GetInfo(ctx, key)
	if err != nil {
		return snapshots.Usage{}, fmt.Errorf("failed to get snapshot info: %w", err)
	}

	if info.Kind == snapshots.KindCommitted {
		// Committed MUST be a cimfs layer
		cimUsage, err := cimfs.GetCimUsage(ctx, s.getLayerCimPath(id))
		if err != nil {
			return snapshots.Usage{}, err
		}
		baseUsage.Size += int64(cimUsage)
	}
	return baseUsage, nil
}

func (s *cimFSSnapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindActive, key, parent, opts)
}

func (s *cimFSSnapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.createSnapshot(ctx, snapshots.KindView, key, parent, opts)
}

func (s *cimFSSnapshotter) Mounts(ctx context.Context, key string) (_ []mount.Mount, err error) {
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

func (s *cimFSSnapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
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
func (s *cimFSSnapshotter) Remove(ctx context.Context, key string) error {
	var ID, renamedID string

	// collect original ID before preRemove
	err := s.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		var infoErr error
		ID, _, _, infoErr = storage.GetInfo(ctx, key)
		return infoErr
	})
	if err != nil {
		return fmt.Errorf("%w: failed to get snapshot info: %s", errdefs.ErrFailedPrecondition, err)
	}

	renamedID, err = s.preRemove(ctx, key)
	if err != nil {
		// wrap as ErrFailedPrecondition so that cleanup of other snapshots can continue
		return fmt.Errorf("%w: %s", errdefs.ErrFailedPrecondition, err)
	}

	if err := cimfs.DestroyCim(ctx, s.getLayerCimPath(ID)); err != nil {
		// Must be cleaned up, any "rm-*" could be removed if no active transactions
		log.G(ctx).WithError(err).WithField("ID", ID).Warnf("failed to cleanup cim files")
	}

	if err = hcsshim.DestroyLayer(s.info, renamedID); err != nil {
		// Must be cleaned up, any "rm-*" could be removed if no active transactions
		log.G(ctx).WithError(err).WithField("renamedID", renamedID).Warnf("failed to remove root filesystem")
	}
	return nil
}

func (s *cimFSSnapshotter) createSnapshot(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) (_ []mount.Mount, err error) {
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

	return s.mounts(newSnapshot, key), nil
}

// In case of CimFS layers, the scratch VHDs are fully empty (WCIFS layers have reparse points in scratch VHDs, hence those VHDs are unique per image), so we create only one scratch VHD and then copy & expand it for every scratch layer creation.
func (s *cimFSSnapshotter) createScratchLayer(ctx context.Context, snDir string, sizeInBytes uint64) error {
	dest := filepath.Join(snDir, "sandbox.vhdx")
	if err := copyScratchDisk(filepath.Join(s.root, templateVHDName), dest); err != nil {
		return err
	}

	if sizeInBytes != 0 {
		if err := hcsshim.ExpandSandboxSize(s.info, filepath.Base(snDir), sizeInBytes); err != nil {
			return fmt.Errorf("failed to expand sandbox vhdx size to %d bytes: %w", sizeInBytes, err)
		}
	}
	return nil
}

func (s *cimFSSnapshotter) mounts(sn storage.Snapshot, key string) []mount.Mount {
	var (
		roFlag string
	)

	if sn.Kind == snapshots.KindView {
		roFlag = "ro"
	} else {
		roFlag = "rw"
	}

	options := []string{
		roFlag,
	}

	// add the layer CIM path - this path will only be used if we are extracting an
	// image layer, in case of scratch snapshots this path MUST be ignored.
	layerCimPath := s.getLayerCimPath(sn.ID)
	options = append(options, mount.LayerCimPathFlag+layerCimPath)

	if len(sn.ParentIDs) != 0 {
		parentLayerPaths := s.parentIDsToParentPaths(sn.ParentIDs)
		parentLayerCimPaths := s.parentIDsToCimPaths(sn.ParentIDs)
		// error is not checked here, as a string array will never fail to Marshal
		parentLayersJSON, _ := json.Marshal(parentLayerPaths)
		parentLayersOption := mount.ParentLayerPathsFlag + string(parentLayersJSON)
		parentCimLayersJSON, _ := json.Marshal(parentLayerCimPaths)
		parentCimLayersOption := mount.ParentLayerCimPathsFlag + string(parentCimLayersJSON)
		options = append(options, parentLayersOption)
		options = append(options, parentCimLayersOption)
	}

	mounts := []mount.Mount{
		{
			Source:  s.getSnapshotDir(sn.ID),
			Type:    mount.CimFSMountType,
			Options: options,
		},
	}

	return mounts
}

// creates a base scratch VHD and a differencing VHD from that base VHD inside the given
// `path` directory. Once these VHDs are created, every scratch snapshot will make a copy
// of the differencing VHD to be used as the scratch for that snapshot. We could ideally
// just have a base VHD and no differencing VHD and copy the base VHD for every scratch
// snapshot. However, base VHDs are slightly bigger in size and so take longer to copy so
// we keep a differencing VHD and copy that.
func createDifferencingScratchVHDs(ctx context.Context, path string) (err error) {
	baseVHDPath := filepath.Join(path, baseVHDName)
	diffVHDPath := filepath.Join(path, templateVHDName)
	baseVHDExists := false
	diffVHDExists := false

	if _, err = os.Stat(baseVHDPath); err == nil {
		baseVHDExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat base VHD: %w", err)
	}

	_, err = os.Stat(diffVHDPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat diff VHD: %w", err)
	} else if baseVHDExists && err == nil {
		diffVHDExists = true
	} else {
		// remove this diff VHD, it must be recreated with the new base VHD.
		os.RemoveAll(diffVHDPath)
	}

	defer func() {
		if err != nil {
			os.RemoveAll(baseVHDPath)
			os.RemoveAll(diffVHDPath)
		}
	}()

	if !baseVHDExists {
		if err = createScratchVHD(ctx, baseVHDPath); err != nil {
			return fmt.Errorf("failed to create base vhd: %w", err)
		}
	}

	if !diffVHDExists {
		// Create the differencing disk that will be what's copied for the final rw layer
		// for a container.
		if err = vhd.CreateDiffVhd(diffVHDPath, baseVHDPath, vhdBlockSizeInBytes); err != nil {
			return fmt.Errorf("failed to create differencing disk: %w", err)
		}
	}

	// Grant VM group access to the differencing VHD (base VHD access is already granted by createSingleVHD)
	if err = security.GrantVmGroupAccess(diffVHDPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %s: %w", diffVHDPath, err)
	}
	return nil
}

// createScratchVHD creates a new scratch VHD at the specified path with the given options
func createScratchVHD(ctx context.Context, vhdPath string, opts ...scratchCreationOpt) (retErr error) {
	// Apply default options
	options := defaultScratchCreationOptions()
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return fmt.Errorf("failed to apply scratch creation option: %w", err)
		}
	}

	// Check if VHD already exists
	if _, err := os.Stat(vhdPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat VHD path: %w", err)
	}

	createParams := &vhd.CreateVirtualDiskParameters{
		Version: 2,
		Version2: vhd.CreateVersion2{
			MaximumSize:      options.sizeInBytes,
			BlockSizeInBytes: vhdBlockSizeInBytes,
		},
	}

	vhdHandle, err := vhd.CreateVirtualDisk(vhdPath, vhd.VirtualDiskAccessNone, vhd.CreateVirtualDiskFlagNone, createParams)
	if err != nil {
		return fmt.Errorf("failed to create VHD: %w", err)
	}
	defer func() {
		if retErr != nil {
			if rmErr := os.RemoveAll(vhdPath); rmErr != nil {
				log.G(ctx).WithError(err).Warnf("on error cleanup failed: %s", rmErr)
			}
		}
	}()

	// Format the VHD if requested
	if options.ntfsFormat {
		err = computestorage.FormatWritableLayerVhd(ctx, windows.Handle(vhdHandle))
	}

	// Always close the handle
	closeErr := syscall.CloseHandle(vhdHandle)

	// Handle errors from formatting and closing
	if err != nil {
		return fmt.Errorf("failed to format VHD: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close VHD handle: %w", closeErr)
	}

	// Grant VM group access to the VHD
	if err = security.GrantVmGroupAccess(vhdPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %s: %w", vhdPath, err)
	}
	return nil
}
