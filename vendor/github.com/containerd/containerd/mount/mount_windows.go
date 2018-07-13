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
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/Microsoft/hcsshim"
	"github.com/pkg/errors"
)

var (
	// ErrNotImplementOnWindows is returned when an action is not implemented for windows
	ErrNotImplementOnWindows = errors.New("not implemented under windows")
)

// Mount to the provided target
func (m *Mount) Mount(target string) error {
	home, layerID := filepath.Split(m.Source)

	parentLayerPaths, err := m.GetParentPaths()
	if err != nil {
		return err
	}

	var di = hcsshim.DriverInfo{
		HomeDir: home,
	}

	if err = hcsshim.ActivateLayer(di, layerID); err != nil {
		return errors.Wrapf(err, "failed to activate layer %s", m.Source)
	}
	defer func() {
		if err != nil {
			hcsshim.DeactivateLayer(di, layerID)
		}
	}()

	if err = hcsshim.PrepareLayer(di, layerID, parentLayerPaths); err != nil {
		return errors.Wrapf(err, "failed to prepare layer %s", m.Source)
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
				return nil, errors.Wrap(err, "failed to unmarshal parent layer paths from mount")
			}
		}
	}
	return parentLayerPaths, nil
}

// Unmount the mount at the provided path
func Unmount(mount string, flags int) error {
	var (
		home, layerID = filepath.Split(mount)
		di            = hcsshim.DriverInfo{
			HomeDir: home,
		}
	)

	if err := hcsshim.UnprepareLayer(di, layerID); err != nil {
		return errors.Wrapf(err, "failed to unprepare layer %s", mount)
	}
	if err := hcsshim.DeactivateLayer(di, layerID); err != nil {
		return errors.Wrapf(err, "failed to deactivate layer %s", mount)
	}

	return nil
}

// UnmountAll unmounts from the provided path
func UnmountAll(mount string, flags int) error {
	return Unmount(mount, flags)
}
