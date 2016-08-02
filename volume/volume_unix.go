// +build linux freebsd darwin solaris

package volume

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/opencontainers/runc/libcontainer/label"
)

// read-write modes
var rwModes = map[string]bool{
	"rw": true,
	"ro": true,
}

// label modes
var labelModes = map[string]bool{
	"Z": true,
	"z": true,
}

const (
	// DefaultCopyMode is the copy mode used by default for normal/named volumes
	DefaultCopyMode = true
)

// MountPoint is the intersection point between a volume and a container. It
// specifies which volume is to be used and where inside a container it should
// be mounted.
type MountPoint struct {
	CommonMountPoint

	// Platform specific fields below here.
	Mode        string `json:"Relabel"` // Originally field was `Relabel`"
	Propagation string // Mount propagation string
	// Specifies if data should be copied from the container before the first mount
	// Use a pointer here so we can tell if the user set this value explicitly
	// This allows us to error out when the user explicitly enabled copy but we can't copy due to the volume being populated
	CopyData bool `json:"-"`
}

// BackwardsCompatible decides whether this mount point can be
// used in old versions of Docker or not.
// Only bind mounts and local volumes can be used in old versions of Docker.
func (m *MountPoint) BackwardsCompatible() bool {
	return len(m.Source) > 0 || m.Driver == DefaultDriverName
}

// HasResource checks whether the given absolute path for a container is in
// this mount point. If the relative path starts with `../` then the resource
// is outside of this mount point, but we can't simply check for this prefix
// because it misses `..` which is also outside of the mount, so check both.
func (m *MountPoint) HasResource(absolutePath string) bool {
	relPath, err := filepath.Rel(m.Destination, absolutePath)
	return err == nil && relPath != ".." && !strings.HasPrefix(relPath, fmt.Sprintf("..%c", filepath.Separator))
}

// ParseMountSpec validates the configuration of mount information is valid.
func ParseMountSpec(spec, volumeDriver string) (*MountPoint, error) {
	spec = filepath.ToSlash(spec)

	mp := &MountPoint{
		CommonMountPoint: CommonMountPoint{
			RW: true,
		},
		Propagation: DefaultPropagationMode,
	}
	if strings.Count(spec, ":") > 2 {
		return nil, errInvalidSpec(spec)
	}

	arr := strings.SplitN(spec, ":", 3)
	if arr[0] == "" {
		return nil, errInvalidSpec(spec)
	}

	switch len(arr) {
	case 1:
		// Just a destination path in the container
		mp.Destination = filepath.Clean(arr[0])
	case 2:
		if isValid := ValidMountMode(arr[1]); isValid {
			// Destination + Mode is not a valid volume - volumes
			// cannot include a mode. eg /foo:rw
			return nil, errInvalidSpec(spec)
		}
		// Host Source Path or Name + Destination
		mp.Source = arr[0]
		mp.Destination = arr[1]
	case 3:
		// HostSourcePath+DestinationPath+Mode
		mp.Source = arr[0]
		mp.Destination = arr[1]
		mp.Mode = arr[2] // Mode field is used by SELinux to decide whether to apply label
		if !ValidMountMode(mp.Mode) {
			return nil, errInvalidMode(mp.Mode)
		}
		mp.RW = ReadWrite(mp.Mode)
		mp.Propagation = GetPropagation(mp.Mode)
	default:
		return nil, errInvalidSpec(spec)
	}

	//validate the volumes destination path
	mp.Destination = filepath.Clean(mp.Destination)
	if !filepath.IsAbs(mp.Destination) {
		return nil, fmt.Errorf("Invalid volume destination path: '%s' mount path must be absolute.", mp.Destination)
	}

	// Destination cannot be "/"
	if mp.Destination == "/" {
		return nil, fmt.Errorf("Invalid specification: destination can't be '/' in '%s'", spec)
	}

	name, source := ParseVolumeSource(mp.Source)
	if len(source) == 0 {
		mp.Source = "" // Clear it out as we previously assumed it was not a name
		mp.Driver = volumeDriver
		// Named volumes can't have propagation properties specified.
		// Their defaults will be decided by docker. This is just a
		// safeguard. Don't want to get into situations where named
		// volumes were mounted as '[r]shared' inside container and
		// container does further mounts under that volume and these
		// mounts become visible on  host and later original volume
		// cleanup becomes an issue if container does not unmount
		// submounts explicitly.
		if HasPropagation(mp.Mode) {
			return nil, errInvalidSpec(spec)
		}
	} else {
		mp.Source = filepath.Clean(source)
	}

	copyData, isSet := getCopyMode(mp.Mode)
	// do not allow copy modes on binds
	if len(name) == 0 && isSet {
		return nil, errInvalidMode(mp.Mode)
	}

	mp.CopyData = copyData
	mp.Name = name

	return mp, nil
}

// ParseVolumeSource parses the origin sources that's mounted into the container.
// It returns a name and a source. It looks to see if the spec passed in
// is an absolute file. If it is, it assumes the spec is a source. If not,
// it assumes the spec is a name.
func ParseVolumeSource(spec string) (string, string) {
	if !filepath.IsAbs(spec) {
		return spec, ""
	}
	return "", spec
}

// IsVolumeNameValid checks a volume name in a platform specific manner.
func IsVolumeNameValid(name string) (bool, error) {
	return true, nil
}

// ValidMountMode will make sure the mount mode is valid.
// returns if it's a valid mount mode or not.
func ValidMountMode(mode string) bool {
	rwModeCount := 0
	labelModeCount := 0
	propagationModeCount := 0
	copyModeCount := 0

	for _, o := range strings.Split(mode, ",") {
		switch {
		case rwModes[o]:
			rwModeCount++
		case labelModes[o]:
			labelModeCount++
		case propagationModes[o]:
			propagationModeCount++
		case copyModeExists(o):
			copyModeCount++
		default:
			return false
		}
	}

	// Only one string for each mode is allowed.
	if rwModeCount > 1 || labelModeCount > 1 || propagationModeCount > 1 || copyModeCount > 1 {
		return false
	}
	return true
}

// ReadWrite tells you if a mode string is a valid read-write mode or not.
// If there are no specifications w.r.t read write mode, then by default
// it returns true.
func ReadWrite(mode string) bool {
	if !ValidMountMode(mode) {
		return false
	}

	for _, o := range strings.Split(mode, ",") {
		if o == "ro" {
			return false
		}
	}

	return true
}

// {<copy mode>=isEnabled}
var copyModes = map[string]bool{
	"nocopy": false,
}

func copyModeExists(mode string) bool {
	_, exists := copyModes[mode]
	return exists
}

// GetCopyMode gets the copy mode from the mode string for mounts
func getCopyMode(mode string) (bool, bool) {
	for _, o := range strings.Split(mode, ",") {
		if isEnabled, exists := copyModes[o]; exists {
			return isEnabled, true
		}
	}
	return DefaultCopyMode, false
}

// relabelIfNeeded is platform specific processing to relabel a bind
func (m *MountPoint) relabelIfNeeded(mountLabel string) error {
	if label.RelabelNeeded(m.Mode) {
		if err := label.Relabel(m.Source, mountLabel, label.IsShared(m.Mode)); err != nil {
			return err
		}
	}
	return nil
}
