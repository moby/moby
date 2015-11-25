package volume

import (
	"os"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/system"
)

// DefaultDriverName is the driver name used for the driver
// implemented in the local package.
const DefaultDriverName string = "local"

// Driver is for creating and removing volumes.
type Driver interface {
	// Name returns the name of the volume driver.
	Name() string
	// Create makes a new volume with the given id.
	Create(name string, opts map[string]string) (Volume, error)
	// Remove deletes the volume.
	Remove(Volume) error
}

// Volume is a place to store data. It is backed by a specific driver, and can be mounted.
type Volume interface {
	// Name returns the name of the volume
	Name() string
	// DriverName returns the name of the driver which owns this volume.
	DriverName() string
	// Path returns the absolute path to the volume.
	Path() string
	// Mount mounts the volume and returns the absolute path to
	// where it can be consumed.
	Mount() (string, error)
	// Unmount unmounts the volume when it is no longer in use.
	Unmount() error
}

// MountPoint is the intersection point between a volume and a container. It
// specifies which volume is to be used and where inside a container it should
// be mounted.
type MountPoint struct {
	Source      string // Container host directory
	Destination string // Inside the container
	RW          bool   // True if writable
	Name        string // Name set by user
	Driver      string // Volume driver to use
	Volume      Volume `json:"-"`

	// Note Mode is not used on Windows
	Mode string `json:"Relabel"` // Originally field was `Relabel`"
}

// Setup sets up a mount point by either mounting the volume if it is
// configured, or creating the source directory if supplied.
func (m *MountPoint) Setup() (string, error) {
	if m.Volume != nil {
		return m.Volume.Mount()
	}
	if len(m.Source) > 0 {
		if _, err := os.Stat(m.Source); err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
			if runtime.GOOS != "windows" { // Windows does not have deprecation issues here
				logrus.Warnf("Auto-creating non-existent volume host path %s, this is deprecated and will be removed soon", m.Source)
				if err := system.MkdirAll(m.Source, 0755); err != nil {
					return "", err
				}
			}
		}
		return m.Source, nil
	}
	return "", derr.ErrorCodeMountSetup
}

// Path returns the path of a volume in a mount point.
func (m *MountPoint) Path() string {
	if m.Volume != nil {
		return m.Volume.Path()
	}
	return m.Source
}

// ValidMountMode will make sure the mount mode is valid.
// returns if it's a valid mount mode or not.
func ValidMountMode(mode string) bool {
	return roModes[strings.ToLower(mode)] || rwModes[strings.ToLower(mode)]
}

// ReadWrite tells you if a mode string is a valid read-write mode or not.
func ReadWrite(mode string) bool {
	return rwModes[strings.ToLower(mode)]
}

// ParseVolumesFrom ensure that the supplied volumes-from is valid.
func ParseVolumesFrom(spec string) (string, string, error) {
	if len(spec) == 0 {
		return "", "", derr.ErrorCodeVolumeFromBlank.WithArgs(spec)
	}

	specParts := strings.SplitN(spec, ":", 2)
	id := specParts[0]
	mode := "rw"

	if len(specParts) == 2 {
		mode = specParts[1]
		if !ValidMountMode(mode) {
			return "", "", derr.ErrorCodeVolumeInvalidMode.WithArgs(mode)
		}
	}
	return id, mode, nil
}

// SplitN splits raw into a maximum of n parts, separated by a separator colon.
// A separator colon is the last `:` character in the regex `[/:\\]?[a-zA-Z]:` (note `\\` is `\` escaped).
// This allows to correctly split strings such as `C:\foo:D:\:rw`.
func SplitN(raw string, n int) []string {
	var array []string
	if len(raw) == 0 || raw[0] == ':' {
		// invalid
		return nil
	}
	// numberOfParts counts the number of parts separated by a separator colon
	numberOfParts := 0
	// left represents the left-most cursor in raw, updated at every `:` character considered as a separator.
	left := 0
	// right represents the right-most cursor in raw incremented with the loop. Note this
	// starts at index 1 as index 0 is already handle above as a special case.
	for right := 1; right < len(raw); right++ {
		// stop parsing if reached maximum number of parts
		if n >= 0 && numberOfParts >= n {
			break
		}
		if raw[right] != ':' {
			continue
		}
		potentialDriveLetter := raw[right-1]
		if (potentialDriveLetter >= 'A' && potentialDriveLetter <= 'Z') || (potentialDriveLetter >= 'a' && potentialDriveLetter <= 'z') {
			if right > 1 {
				beforePotentialDriveLetter := raw[right-2]
				if beforePotentialDriveLetter != ':' && beforePotentialDriveLetter != '/' && beforePotentialDriveLetter != '\\' {
					// e.g. `C:` is not preceded by any delimiter, therefore it was not a drive letter but a path ending with `C:`.
					array = append(array, raw[left:right])
					left = right + 1
					numberOfParts++
				}
				// else, `C:` is considered as a drive letter and not as a delimiter, so we continue parsing.
			}
			// if right == 1, then `C:` is the beginning of the raw string, therefore `:` is again not considered a delimiter and we continue parsing.
		} else {
			// if `:` is not preceded by a potential drive letter, then consider it as a delimiter.
			array = append(array, raw[left:right])
			left = right + 1
			numberOfParts++
		}
	}
	// need to take care of the last part
	if left < len(raw) {
		if n >= 0 && numberOfParts >= n {
			// if the maximum number of parts is reached, just append the rest to the last part
			// left-1 is at the last `:` that needs to be included since not considered a separator.
			array[n-1] += raw[left-1:]
		} else {
			array = append(array, raw[left:])
		}
	}
	return array
}
