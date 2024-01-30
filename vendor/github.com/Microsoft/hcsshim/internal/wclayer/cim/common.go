//go:build windows

package cim

import (
	"os"
	"path/filepath"
)

const (
	// name of the directory in which cims are stored
	cimDir = "cim-layers"
)

// Usually layers are stored at ./root/io.containerd.snapshotter.v1.windows/snapshots/<layerid>. For cimfs we
// must store all layer cims in the same directory (for forked cims to work). So all cim layers are stored in
// /root/io.containerd.snapshotter.v1.windows/snapshots/cim-layers. And the cim file representing each
// individual layer is stored at /root/io.containerd.snapshotter.v1.windows/snapshots/cim-layers/<layerid>.cim

// CimName is the filename (<layerid>.cim) of the file representing the cim
func GetCimNameFromLayer(layerPath string) string {
	return filepath.Base(layerPath) + ".cim"
}

// CimPath is the path to the CimDir/<layerid>.cim file that represents a layer cim.
func GetCimPathFromLayer(layerPath string) string {
	return filepath.Join(GetCimDirFromLayer(layerPath), GetCimNameFromLayer(layerPath))
}

// CimDir is the directory inside which all cims are stored.
func GetCimDirFromLayer(layerPath string) string {
	dir := filepath.Dir(layerPath)
	return filepath.Join(dir, cimDir)
}

// IsCimLayer returns `true` if the layer at path `layerPath` is a cim layer. Returns `false` otherwise.
func IsCimLayer(layerPath string) bool {
	cimPath := GetCimPathFromLayer(layerPath)
	_, err := os.Stat(cimPath)
	return (err == nil)
}
