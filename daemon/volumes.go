package daemon

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/volume"
)

// ErrVolumeReadonly is used to signal an error when trying to copy data into
// a volume mount that is not writable.
var ErrVolumeReadonly = errors.New("mounted volume is marked read-only")

// mountPoint is the intersection point between a volume and a container. It
// specifies which volume is to be used and where inside a container it should
// be mounted.
type mountPoint struct {
	Name        string
	Destination string
	Driver      string
	RW          bool
	Volume      volume.Volume `json:"-"`
	Source      string
	Mode        string `json:"Relabel"` // Originally field was `Relabel`"
}

// Setup sets up a mount point by either mounting the volume if it is
// configured, or creating the source directory if supplied.
func (m *mountPoint) Setup() (string, error) {
	if m.Volume != nil {
		return m.Volume.Mount()
	}

	if len(m.Source) > 0 {
		if _, err := os.Stat(m.Source); err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
			if err := system.MkdirAll(m.Source, 0755); err != nil {
				return "", err
			}
		}
		return m.Source, nil
	}

	return "", fmt.Errorf("Unable to setup mount point, neither source nor volume defined")
}

// hasResource checks whether the given absolute path for a container is in
// this mount point. If the relative path starts with `../` then the resource
// is outside of this mount point, but we can't simply check for this prefix
// because it misses `..` which is also outside of the mount, so check both.
func (m *mountPoint) hasResource(absolutePath string) bool {
	relPath, err := filepath.Rel(m.Destination, absolutePath)

	return err == nil && relPath != ".." && !strings.HasPrefix(relPath, fmt.Sprintf("..%c", filepath.Separator))
}

// Path returns the path of a volume in a mount point.
func (m *mountPoint) Path() string {
	if m.Volume != nil {
		return m.Volume.Path()
	}

	return m.Source
}

// copyExistingContents copies from the source to the destination and
// ensures the ownership is appropriately set.
func copyExistingContents(source, destination string) error {
	volList, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}
	if len(volList) > 0 {
		srcList, err := ioutil.ReadDir(destination)
		if err != nil {
			return err
		}
		if len(srcList) == 0 {
			// If the source volume is empty copy files from the root into the volume
			if err := chrootarchive.CopyWithTar(source, destination); err != nil {
				return err
			}
		}
	}
	return copyOwnership(source, destination)
}
