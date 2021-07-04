// Package computestorage is a wrapper around the HCS storage APIs. These are new storage APIs introduced
// separate from the original graphdriver calls intended to give more freedom around creating
// and managing container layers and scratch spaces.
package computestorage

import (
	hcsschema "github.com/Microsoft/hcsshim/internal/schema2"
)

//go:generate go run ../mksyscall_windows.go -output zsyscall_windows.go storage.go

//sys hcsImportLayer(layerPath string, sourceFolderPath string, layerData string) (hr error) = computestorage.HcsImportLayer?
//sys hcsExportLayer(layerPath string, exportFolderPath string, layerData string, options string) (hr error) = computestorage.HcsExportLayer?
//sys hcsDestroyLayer(layerPath string) (hr error) = computestorage.HcsDestoryLayer?
//sys hcsSetupBaseOSLayer(layerPath string, handle windows.Handle, options string) (hr error) = computestorage.HcsSetupBaseOSLayer?
//sys hcsInitializeWritableLayer(writableLayerPath string, layerData string, options string) (hr error) = computestorage.HcsInitializeWritableLayer?
//sys hcsAttachLayerStorageFilter(layerPath string, layerData string) (hr error) = computestorage.HcsAttachLayerStorageFilter?
//sys hcsDetachLayerStorageFilter(layerPath string) (hr error) = computestorage.HcsDetachLayerStorageFilter?
//sys hcsFormatWritableLayerVhd(handle windows.Handle) (hr error) = computestorage.HcsFormatWritableLayerVhd?
//sys hcsGetLayerVhdMountPath(vhdHandle windows.Handle, mountPath **uint16) (hr error) = computestorage.HcsGetLayerVhdMountPath?
//sys hcsSetupBaseOSVolume(layerPath string, volumePath string, options string) (hr error) = computestorage.HcsSetupBaseOSVolume?

// LayerData is the data used to describe parent layer information.
type LayerData struct {
	SchemaVersion hcsschema.Version `json:"SchemaVersion,omitempty"`
	Layers        []hcsschema.Layer `json:"Layers,omitempty"`
}

// ExportLayerOptions are the set of options that are used with the `computestorage.HcsExportLayer` syscall.
type ExportLayerOptions struct {
	IsWritableLayer bool `json:"IsWritableLayer,omitempty"`
}

// OsLayerType is the type of layer being operated on.
type OsLayerType string

const (
	// OsLayerTypeContainer is a container layer.
	OsLayerTypeContainer OsLayerType = "Container"
	// OsLayerTypeVM is a virtual machine layer.
	OsLayerTypeVM OsLayerType = "Vm"
)

// OsLayerOptions are the set of options that are used with the `SetupBaseOSLayer` and
// `SetupBaseOSVolume` calls.
type OsLayerOptions struct {
	Type                       OsLayerType `json:"Type,omitempty"`
	DisableCiCacheOptimization bool        `json:"DisableCiCacheOptimization,omitempty"`
	SkipUpdateBcdForBoot       bool        `json:"SkipUpdateBcdForBoot,omitempty"`
}
