package mounts // import "github.com/docker/docker/volume/mounts"

import (
	"errors"
	"os"

	"github.com/docker/docker/api/types/mount"
)

const (
	// OSLinux is the same as runtime.GOOS on linux
	OSLinux = "linux"
	// OSWindows is the same as runtime.GOOS on windows
	OSWindows = "windows"
)

// ErrVolumeTargetIsRoot is returned when the target destination is root.
// It's used by both LCOW and Linux parsers.
var ErrVolumeTargetIsRoot = errors.New("invalid specification: destination can't be '/'")

// Parser represents a platform specific parser for mount expressions
type Parser interface {
	ParseMountRaw(raw, volumeDriver string) (*MountPoint, error)
	ParseMountSpec(cfg mount.Mount) (*MountPoint, error)
	ParseVolumesFrom(spec string) (string, string, error)
	DefaultPropagationMode() mount.Propagation
	ConvertTmpfsOptions(opt *mount.TmpfsOptions, readOnly bool) (string, error)
	DefaultCopyMode() bool
	ValidateVolumeName(name string) error
	ReadWrite(mode string) bool
	IsBackwardCompatible(m *MountPoint) bool
	HasResource(m *MountPoint, absPath string) bool
	ValidateTmpfsMountDestination(dest string) error
	ValidateMountConfig(mt *mount.Mount) error
}

type fileInfoProvider interface {
	fileInfo(path string) (exist, isDir bool, err error)
}

type defaultFileInfoProvider struct {
}

func (defaultFileInfoProvider) fileInfo(path string) (exist, isDir bool, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, false, err
		}
		return false, false, nil
	}
	return true, fi.IsDir(), nil
}

var currentFileInfoProvider fileInfoProvider = defaultFileInfoProvider{}

// read-write modes
var rwModes = map[string]bool{
	"rw": true,
	"ro": true,
}
