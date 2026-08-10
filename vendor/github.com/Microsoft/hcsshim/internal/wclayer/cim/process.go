//go:build windows

package cim

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"golang.org/x/sys/windows"
)

// processUtilityVMLayer will handle processing of UVM specific files when we start
// supporting UVM based containers with CimFS in the future.
func processUtilityVMLayer(ctx context.Context, layerPath string) error {
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
func (cw *cimLayerWriter) processBaseLayer(ctx context.Context, processUtilityVM bool) (err error) {
	if processUtilityVM {
		if err = processUtilityVMLayer(ctx, cw.layerPath); err != nil {
			return fmt.Errorf("process utilityVM layer: %w", err)
		}
	}

	ops, err := processBaseLayerHives(cw.layerPath)
	if err != nil {
		return err
	}
	cw.pendingOps = append(cw.pendingOps, ops...)

	ops, err = processLayoutFile(cw.layerPath)
	if err != nil {
		return err
	}
	cw.pendingOps = append(cw.pendingOps, ops...)
	return nil
}

// processNonBaseLayer takes care of the processing required for a non base layer. As of now
// the only processing required for non base layer is to merge the delta registry hives of the
// non-base layer with it's parent layer.
func (cw *cimLayerWriter) processNonBaseLayer(ctx context.Context, processUtilityVM bool) (err error) {
	for _, hv := range hives {
		baseHive := filepath.Join(wclayer.HivesPath, hv.base)
		deltaHive := filepath.Join(wclayer.HivesPath, hv.delta)
		_, err := os.Stat(filepath.Join(cw.layerPath, deltaHive))
		// merge with parent layer if delta exists.
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("stat delta hive %s: %w", filepath.Join(cw.layerPath, deltaHive), err)
		} else if err == nil {
			// merge base hive of parent layer with the delta hive of this layer and write it as
			// the base hive of this layer.
			err = mergeHive(filepath.Join(cw.parentLayerPaths[0], baseHive), filepath.Join(cw.layerPath, deltaHive), filepath.Join(cw.layerPath, baseHive))
			if err != nil {
				return err
			}

			// the newly created merged file must be added to the cim
			cw.pendingOps = append(cw.pendingOps, &addOp{
				pathInCim: baseHive,
				hostPath:  filepath.Join(cw.layerPath, baseHive),
				fileInfo: &winio.FileBasicInfo{
					FileAttributes: windows.FILE_ATTRIBUTE_NORMAL,
				},
			})
		}
	}

	if processUtilityVM {
		return processUtilityVMLayer(ctx, cw.layerPath)
	}
	return nil
}
