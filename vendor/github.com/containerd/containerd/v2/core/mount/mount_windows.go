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

package mount

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio/pkg/bindfilter"
	"github.com/Microsoft/hcsshim"
	"github.com/containerd/log"
	"golang.org/x/sys/windows"
)

const sourceStreamName = "containerd.io-source"

// Mount to the provided target.
func (m *Mount) mount(target string) (retErr error) {
	if m.Type != "windows-layer" {
		return fmt.Errorf("invalid windows mount type: '%s'", m.Type)
	}

	home, layerID := filepath.Split(m.Source)

	parentLayerPaths, err := m.GetParentPaths()
	if err != nil {
		return err
	}

	var di = hcsshim.DriverInfo{
		HomeDir: home,
	}

	if err := hcsshim.ActivateLayer(di, layerID); err != nil {
		return fmt.Errorf("failed to activate layer %s: %w", m.Source, err)
	}
	defer func() {
		if retErr != nil {
			if layerErr := hcsshim.DeactivateLayer(di, layerID); layerErr != nil {
				log.G(context.TODO()).WithError(layerErr).Error("failed to deactivate layer during mount failure cleanup")
			}
		}
	}()

	if err := hcsshim.PrepareLayer(di, layerID, parentLayerPaths); err != nil {
		return fmt.Errorf("failed to prepare layer %s: %w", m.Source, err)
	}

	defer func() {
		if retErr != nil {
			if layerErr := hcsshim.UnprepareLayer(di, layerID); layerErr != nil {
				log.G(context.TODO()).WithError(layerErr).Error("failed to unprepare layer during mount failure cleanup")
			}
		}
	}()

	volume, err := hcsshim.GetLayerMountPath(di, layerID)
	if err != nil {
		return fmt.Errorf("failed to get volume path for layer %s: %w", m.Source, err)
	}

	if len(parentLayerPaths) == 0 {
		// this is a base layer. It gets mounted without going through WCIFS. We need to mount the Files
		// folder, not the actual source, or the client may inadvertently remove metadata files.
		volume = filepath.Join(volume, "Files")
		if _, err := os.Stat(volume); err != nil {
			return fmt.Errorf("no Files folder in layer %s", layerID)
		}
	}
	if err := bindfilter.ApplyFileBinding(target, volume, m.ReadOnly()); err != nil {
		return fmt.Errorf("failed to set volume mount path for layer %s: %w", m.Source, err)
	}
	defer func() {
		if retErr != nil {
			if bindErr := bindfilter.RemoveFileBinding(target); bindErr != nil {
				log.G(context.TODO()).WithError(bindErr).Error("failed to remove binding during mount failure cleanup")
			}
		}
	}()

	// Add an Alternate Data Stream to record the layer source.
	// See https://docs.microsoft.com/en-au/archive/blogs/askcore/alternate-data-streams-in-ntfs
	// for details on Alternate Data Streams.
	if err := os.WriteFile(filepath.Clean(target)+":"+sourceStreamName, []byte(m.Source), 0666); err != nil {
		return fmt.Errorf("failed to record source for layer %s: %w", m.Source, err)
	}

	return nil
}

// ParentLayerPathsFlag is the options flag used to represent the JSON encoded
// list of parent layers required to use the layer
const ParentLayerPathsFlag = "parentLayerPaths="

// GetParentPaths of the mount
func (m *Mount) GetParentPaths() ([]string, error) {
	var parentLayerPaths []string
	for _, option := range m.Options {
		if strings.HasPrefix(option, ParentLayerPathsFlag) {
			err := json.Unmarshal([]byte(option[len(ParentLayerPathsFlag):]), &parentLayerPaths)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal parent layer paths from mount: %w", err)
			}
		}
	}
	return parentLayerPaths, nil
}

// Unmount the mount at the provided path
func Unmount(mount string, flags int) error {
	mount = filepath.Clean(mount)
	adsFile := mount + ":" + sourceStreamName
	var layerPath string

	if _, err := os.Lstat(adsFile); err == nil {
		layerPathb, err := os.ReadFile(mount + ":" + sourceStreamName)
		if err != nil {
			return fmt.Errorf("failed to retrieve source for layer %s: %w", mount, err)
		}
		layerPath = string(layerPathb)
	}

	if err := bindfilter.RemoveFileBinding(mount); err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_NOT_FOUND) {
			// not a mount point
			return nil
		}
		return fmt.Errorf("removing mount: %w", err)
	}

	if layerPath != "" {
		var (
			home, layerID = filepath.Split(layerPath)
			di            = hcsshim.DriverInfo{
				HomeDir: home,
			}
		)

		if err := hcsshim.UnprepareLayer(di, layerID); err != nil {
			return fmt.Errorf("failed to unprepare layer %s: %w", mount, err)
		}

		if err := hcsshim.DeactivateLayer(di, layerID); err != nil {
			return fmt.Errorf("failed to deactivate layer %s: %w", mount, err)
		}
	}
	return nil
}

// UnmountAll unmounts from the provided path
func UnmountAll(mount string, flags int) error {
	if mount == "" {
		// This isn't an error, per the EINVAL handling in the Linux version
		return nil
	}
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		return nil
	}

	return Unmount(mount, flags)
}

// UnmountRecursive unmounts from the provided path
func UnmountRecursive(mount string, flags int) error {
	return UnmountAll(mount, flags)
}

// CimFS specific constants
const (
	// LayerCimPathFlag is the option flag used to represent the path at which a layer
	// CIM must be stored. This flag only applies if an image layer is being extracted
	// onto the snapshot i.e the snapshot key has an UnpackKeyPrefix. For all other
	// snapshots, this MUST be ignored.
	LayerCimPathFlag = "cimpath="

	// Similar to ParentLayerPathsFlag this is the optinos flag used to represent the JSON encoded list of
	// parent layer CIMs
	ParentLayerCimPathsFlag = "parentCimPaths="

	// string to specify the standard cimfs type of mount
	CimFSMountType string = "CimFS"

	// flag that specifies what kind of a block CIM is being used in the snapshot
	BlockCIMTypeFlag string = "blockCIMType="
	// "file" type of block CIM that will usually be used in snapshots
	BlockCIMTypeFile string = "file"
	// flag that specifies the path of a merged block CIM.
	MergedCIMPathFlag string = "mergedCIMPath="
	// a block CIM snapshotter mount type
	BlockCIMMountType string = "BlockCIM"
	// flag that enables layer integrity for block CIMs
	EnableLayerIntegrityFlag string = "enable_layer_integrity"
	// flag that enables VHD footer for block CIMs
	AppendVHDFooterFlag string = "append_vhd_footer"
)

// getOptionByPrefix finds an option that has the provided prefix, cuts the prefix from
// that option string and return the remaining string. Boolean return is set to true if an
// option is found with the given prefix. It is set to false otherwise.
func getOptionByPrefix(m *Mount, prefix string) (string, bool) {
	for _, option := range m.Options {
		val, found := strings.CutPrefix(option, prefix)
		if found {
			return val, true
		}
	}
	return "", false
}

// gets the paths of the parent cims of this mount
func GetParentCimPaths(m *Mount) ([]string, error) {
	if m.Type != CimFSMountType {
		return nil, fmt.Errorf("invalid mount type: '%s'", m.Type)
	}
	var parentCimPaths []string
	val, found := getOptionByPrefix(m, ParentLayerCimPathsFlag)
	if !found {
		return parentCimPaths, nil
	}
	err := json.Unmarshal([]byte(val), &parentCimPaths)
	return parentCimPaths, err
}

// Only applies to a snapshot created for image extraction, for such a snapshot provides the
// path to a cim in which image layer will be extracted.
func GetCimPath(m *Mount) (string, error) {
	if m.Type != CimFSMountType {
		return "", fmt.Errorf("invalid mount type: '%s'", m.Type)
	}
	cimPath, found := getOptionByPrefix(m, LayerCimPathFlag)
	if !found {
		return "", fmt.Errorf("cim path not found")
	}
	return cimPath, nil

}

// GetEnableLayerIntegrity checks if the enableLayerIntegrity flag is present in mount options
func GetEnableLayerIntegrity(m *Mount) bool {
	for _, option := range m.Options {
		if option == EnableLayerIntegrityFlag {
			return true
		}
	}
	return false
}

// GetAppendVHDFooter checks if the appendVHDFooter flag is present in mount options
func GetAppendVHDFooter(m *Mount) bool {
	for _, option := range m.Options {
		if option == AppendVHDFooterFlag {
			return true
		}
	}
	return false
}
