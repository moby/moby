package volume // import "github.com/docker/docker/volume"

import (
	"errors"
	"runtime"

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

// NewParser creates a parser for a given container OS, depending on the current host OS (linux on a windows host will resolve to an lcowParser)
func NewParser(containerOS string) Parser {
	switch containerOS {
	case OSWindows:
		return &windowsParser{}
	}
	if runtime.GOOS == OSWindows {
		return &lcowParser{}
	}
	return &linuxParser{}
}
