//go:build windows

package cim

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/vhd"
	"github.com/Microsoft/hcsshim/computestorage"
	"github.com/Microsoft/hcsshim/internal/memory"
	"github.com/Microsoft/hcsshim/internal/security"
	"github.com/Microsoft/hcsshim/internal/vhdx"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"golang.org/x/sys/windows"
)

const defaultVHDXBlockSizeInMB = 1

func createContainerBaseLayerVHDs(ctx context.Context, layerPath string) (err error) {
	baseVhdPath := filepath.Join(layerPath, wclayer.ContainerBaseVhd)
	diffVhdPath := filepath.Join(layerPath, wclayer.ContainerScratchVhd)
	defaultVhdSize := uint64(20)

	if _, err := os.Stat(baseVhdPath); err == nil {
		if err := os.RemoveAll(baseVhdPath); err != nil {
			return fmt.Errorf("failed to remove base vhdx path:  %w", err)
		}
	}
	if _, err := os.Stat(diffVhdPath); err == nil {
		if err := os.RemoveAll(diffVhdPath); err != nil {
			return fmt.Errorf("failed to remove differencing vhdx: %w", err)
		}
	}

	createParams := &vhd.CreateVirtualDiskParameters{
		Version: 2,
		Version2: vhd.CreateVersion2{
			MaximumSize:      defaultVhdSize * memory.GiB,
			BlockSizeInBytes: defaultVHDXBlockSizeInMB * memory.MiB,
		},
	}
	handle, err := vhd.CreateVirtualDisk(baseVhdPath, vhd.VirtualDiskAccessNone, vhd.CreateVirtualDiskFlagNone, createParams)
	if err != nil {
		return fmt.Errorf("failed to create vhdx: %w", err)
	}

	defer func() {
		if err != nil {
			os.RemoveAll(baseVhdPath)
			os.RemoveAll(diffVhdPath)
		}
	}()

	err = computestorage.FormatWritableLayerVhd(ctx, windows.Handle(handle))
	// we always wanna close the handle whether format succeeds for not.
	closeErr := syscall.CloseHandle(handle)
	if err != nil {
		return err
	} else if closeErr != nil {
		return fmt.Errorf("failed to close vhdx handle: %w", closeErr)
	}

	// Create the differencing disk that will be what's copied for the final rw layer
	// for a container.
	if err = vhd.CreateDiffVhd(diffVhdPath, baseVhdPath, defaultVHDXBlockSizeInMB); err != nil {
		return fmt.Errorf("failed to create differencing disk: %w", err)
	}

	if err = security.GrantVmGroupAccess(baseVhdPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %s: %w", baseVhdPath, err)
	}
	if err = security.GrantVmGroupAccess(diffVhdPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %s: %w", diffVhdPath, err)
	}
	return nil
}

// processUtilityVMLayer is similar to createContainerBaseLayerVHDs but along with the scratch creation it
// also does some BCD modifications to allow the UVM to boot from the CIM. It expects that the UVM BCD file is
// present at layerPath/`wclayer.BcdFilePath` and a UVM SYSTEM hive is present at
// layerPath/UtilityVM/`wclayer.RegFilesPath`/SYSTEM. The scratch VHDs are created under the `layerPath`
// directory.
func processUtilityVMLayer(ctx context.Context, layerPath string) error {
	// func createUtilityVMLayerVHDs(ctx context.Context, layerPath string) error {
	baseVhdPath := filepath.Join(layerPath, wclayer.UtilityVMPath, wclayer.UtilityVMBaseVhd)
	diffVhdPath := filepath.Join(layerPath, wclayer.UtilityVMPath, wclayer.UtilityVMScratchVhd)
	defaultVhdSize := uint64(10)

	// Just create the vhdx for utilityVM layer, no need to format it.
	createParams := &vhd.CreateVirtualDiskParameters{
		Version: 2,
		Version2: vhd.CreateVersion2{
			MaximumSize:      defaultVhdSize * memory.GiB,
			BlockSizeInBytes: defaultVHDXBlockSizeInMB * memory.MiB,
		},
	}

	handle, err := vhd.CreateVirtualDisk(baseVhdPath, vhd.VirtualDiskAccessNone, vhd.CreateVirtualDiskFlagNone, createParams)
	if err != nil {
		return fmt.Errorf("failed to create vhdx: %w", err)
	}

	defer func() {
		if err != nil {
			os.RemoveAll(baseVhdPath)
			os.RemoveAll(diffVhdPath)
		}
	}()

	err = computestorage.FormatWritableLayerVhd(ctx, windows.Handle(handle))
	closeErr := syscall.CloseHandle(handle)
	if err != nil {
		return err
	} else if closeErr != nil {
		return fmt.Errorf("failed to close vhdx handle: %w", closeErr)
	}

	partitionInfo, err := vhdx.GetScratchVhdPartitionInfo(ctx, baseVhdPath)
	if err != nil {
		return fmt.Errorf("failed to get base vhd layout info: %w", err)
	}
	// relativeCimPath needs to be the cim path relative to the snapshots directory. The snapshots
	// directory is shared inside the UVM over VSMB, so during the UVM boot this relative path will be
	// used to find the cim file under that VSMB share.
	relativeCimPath := filepath.Join(filepath.Base(GetCimDirFromLayer(layerPath)), GetCimNameFromLayer(layerPath))
	bcdPath := filepath.Join(layerPath, bcdFilePath)
	if err = updateBcdStoreForBoot(bcdPath, relativeCimPath, partitionInfo.DiskID, partitionInfo.PartitionID); err != nil {
		return fmt.Errorf("failed to update BCD: %w", err)
	}

	if err := enableCimBoot(filepath.Join(layerPath, wclayer.UtilityVMPath, wclayer.RegFilesPath, "SYSTEM")); err != nil {
		return fmt.Errorf("failed to setup cim image for uvm boot: %w", err)
	}

	// Note: diff vhd creation and granting of vm group access must be done AFTER
	// getting the partition info of the base VHD. Otherwise it causes the vhd parent
	// chain to get corrupted.
	// TODO(ambarve): figure out why this happens so that bcd update can be moved to a separate function

	// Create the differencing disk that will be what's copied for the final rw layer
	// for a container.
	if err = vhd.CreateDiffVhd(diffVhdPath, baseVhdPath, defaultVHDXBlockSizeInMB); err != nil {
		return fmt.Errorf("failed to create differencing disk: %w", err)
	}

	if err := security.GrantVmGroupAccess(baseVhdPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %s: %w", baseVhdPath, err)
	}
	if err := security.GrantVmGroupAccess(diffVhdPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %s: %w", diffVhdPath, err)
	}
	return nil
}

// processBaseLayerHives make the base layer specific modifications on the hives and emits equivalent the
// pendingCimOps that should be applied on the CIM.  In base layer we need to create hard links from registry
// hives under Files/Windows/Sysetm32/config into Hives/*_BASE. This function creates these links outside so
// that the registry hives under Hives/ are available during children layers import.  Then we write these hive
// files inside the cim and create links inside the cim.
func processBaseLayerHives(layerPath string) ([]pendingCimOp, error) {
	pendingOps := []pendingCimOp{}

	// make hives directory both outside and in the cim
	if err := os.Mkdir(filepath.Join(layerPath, wclayer.HivesPath), 0755); err != nil {
		return pendingOps, fmt.Errorf("hives directory creation: %w", err)
	}

	hivesDirInfo := &winio.FileBasicInfo{
		CreationTime:   windows.NsecToFiletime(time.Now().UnixNano()),
		LastAccessTime: windows.NsecToFiletime(time.Now().UnixNano()),
		LastWriteTime:  windows.NsecToFiletime(time.Now().UnixNano()),
		ChangeTime:     windows.NsecToFiletime(time.Now().UnixNano()),
		FileAttributes: windows.FILE_ATTRIBUTE_DIRECTORY,
	}
	pendingOps = append(pendingOps, &addOp{
		pathInCim: wclayer.HivesPath,
		hostPath:  filepath.Join(layerPath, wclayer.HivesPath),
		fileInfo:  hivesDirInfo,
	})

	// add hard links from base hive files.
	for _, hv := range hives {
		oldHivePathRelative := filepath.Join(wclayer.RegFilesPath, hv.name)
		newHivePathRelative := filepath.Join(wclayer.HivesPath, hv.base)
		if err := os.Link(filepath.Join(layerPath, oldHivePathRelative), filepath.Join(layerPath, newHivePathRelative)); err != nil {
			return pendingOps, fmt.Errorf("hive link creation: %w", err)
		}

		pendingOps = append(pendingOps, &linkOp{
			oldPath: oldHivePathRelative,
			newPath: newHivePathRelative,
		})
	}
	return pendingOps, nil
}

// processLayoutFile creates a file named "layout" in the root of the base layer. This allows certain
// container startup related functions to understand that the hives are a part of the container rootfs.
func processLayoutFile(layerPath string) ([]pendingCimOp, error) {
	fileContents := "vhd-with-hives\n"
	if err := os.WriteFile(filepath.Join(layerPath, "layout"), []byte(fileContents), 0755); err != nil {
		return []pendingCimOp{}, fmt.Errorf("write layout file: %w", err)
	}

	layoutFileInfo := &winio.FileBasicInfo{
		CreationTime:   windows.NsecToFiletime(time.Now().UnixNano()),
		LastAccessTime: windows.NsecToFiletime(time.Now().UnixNano()),
		LastWriteTime:  windows.NsecToFiletime(time.Now().UnixNano()),
		ChangeTime:     windows.NsecToFiletime(time.Now().UnixNano()),
		FileAttributes: windows.FILE_ATTRIBUTE_NORMAL,
	}

	op := &addOp{
		pathInCim: "layout",
		hostPath:  filepath.Join(layerPath, "layout"),
		fileInfo:  layoutFileInfo,
	}
	return []pendingCimOp{op}, nil
}

// Some of the layer files that are generated during the processBaseLayer call must be added back
// inside the cim, some registry file links must be updated. This function takes care of all those
// steps. This function opens the cim file for writing and updates it.
func (cw *CimLayerWriter) processBaseLayer(ctx context.Context, processUtilityVM bool) (err error) {
	if err = createContainerBaseLayerVHDs(ctx, cw.path); err != nil {
		return fmt.Errorf("failed to create container base VHDs: %w", err)
	}

	if processUtilityVM {
		if err = processUtilityVMLayer(ctx, cw.path); err != nil {
			return fmt.Errorf("process utilityVM layer: %w", err)
		}
	}

	ops, err := processBaseLayerHives(cw.path)
	if err != nil {
		return err
	}
	cw.pendingOps = append(cw.pendingOps, ops...)

	ops, err = processLayoutFile(cw.path)
	if err != nil {
		return err
	}
	cw.pendingOps = append(cw.pendingOps, ops...)
	return nil
}

// processNonBaseLayer takes care of the processing required for a non base layer. As of now
// the only processing required for non base layer is to merge the delta registry hives of the
// non-base layer with it's parent layer.
func (cw *CimLayerWriter) processNonBaseLayer(ctx context.Context, processUtilityVM bool) (err error) {
	for _, hv := range hives {
		baseHive := filepath.Join(wclayer.HivesPath, hv.base)
		deltaHive := filepath.Join(wclayer.HivesPath, hv.delta)
		_, err := os.Stat(filepath.Join(cw.path, deltaHive))
		// merge with parent layer if delta exists.
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("stat delta hive %s: %w", filepath.Join(cw.path, deltaHive), err)
		} else if err == nil {
			// merge base hive of parent layer with the delta hive of this layer and write it as
			// the base hive of this layer.
			err = mergeHive(filepath.Join(cw.parentLayerPaths[0], baseHive), filepath.Join(cw.path, deltaHive), filepath.Join(cw.path, baseHive))
			if err != nil {
				return err
			}

			// the newly created merged file must be added to the cim
			cw.pendingOps = append(cw.pendingOps, &addOp{
				pathInCim: baseHive,
				hostPath:  filepath.Join(cw.path, baseHive),
				fileInfo: &winio.FileBasicInfo{
					CreationTime:   windows.NsecToFiletime(time.Now().UnixNano()),
					LastAccessTime: windows.NsecToFiletime(time.Now().UnixNano()),
					LastWriteTime:  windows.NsecToFiletime(time.Now().UnixNano()),
					ChangeTime:     windows.NsecToFiletime(time.Now().UnixNano()),
					FileAttributes: windows.FILE_ATTRIBUTE_NORMAL,
				},
			})
		}
	}

	if processUtilityVM {
		return processUtilityVMLayer(ctx, cw.path)
	}
	return nil
}
