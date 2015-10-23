// +build linux freebsd darwin

package volume

import (
	"fmt"
	"path/filepath"
	"strings"

	derr "github.com/docker/docker/errors"
)

// read-write modes
var rwModes = map[string]bool{
	"rw":   true,
	"rw,Z": true,
	"rw,z": true,
	"z,rw": true,
	"Z,rw": true,
	"Z":    true,
	"z":    true,
}

// read-only modes
var roModes = map[string]bool{
	"ro":   true,
	"ro,Z": true,
	"ro,z": true,
	"z,ro": true,
	"Z,ro": true,
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
		RW: true,
	}
	if strings.Count(spec, ":") > 2 {
		return nil, derr.ErrorCodeVolumeInvalid.WithArgs(spec)
	}

	arr := strings.SplitN(spec, ":", 3)
	if arr[0] == "" {
		return nil, derr.ErrorCodeVolumeInvalid.WithArgs(spec)
	}

	switch len(arr) {
	case 1:
		// Just a destination path in the container
		mp.Destination = filepath.Clean(arr[0])
	case 2:
		if isValid := ValidMountMode(arr[1]); isValid {
			// Destination + Mode is not a valid volume - volumes
			// cannot include a mode. eg /foo:rw
			return nil, derr.ErrorCodeVolumeInvalid.WithArgs(spec)
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
			return nil, derr.ErrorCodeVolumeInvalidMode.WithArgs(mp.Mode)
		}
		mp.RW = ReadWrite(mp.Mode)
	default:
		return nil, derr.ErrorCodeVolumeInvalid.WithArgs(spec)
	}

	//validate the volumes destination path
	mp.Destination = filepath.Clean(mp.Destination)
	if !filepath.IsAbs(mp.Destination) {
		return nil, derr.ErrorCodeVolumeAbs.WithArgs(mp.Destination)
	}

	// Destination cannot be "/"
	if mp.Destination == "/" {
		return nil, derr.ErrorCodeVolumeSlash.WithArgs(spec)
	}

	name, source := ParseVolumeSource(mp.Source)
	if len(source) == 0 {
		mp.Source = "" // Clear it out as we previously assumed it was not a name
		mp.Driver = volumeDriver
		if len(mp.Driver) == 0 {
			mp.Driver = DefaultDriverName
		}
	} else {
		mp.Source = filepath.Clean(source)
	}

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
