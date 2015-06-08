// +build !windows

package daemon

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/system"
)

// copyOwnership copies the permissions and uid:gid of the source file
// into the destination file
func copyOwnership(source, destination string) error {
	stat, err := system.Stat(source)
	if err != nil {
		return err
	}

	if err := os.Chown(destination, int(stat.Uid()), int(stat.Gid())); err != nil {
		return err
	}

	return os.Chmod(destination, os.FileMode(stat.Mode()))
}

func (container *Container) setupMounts() ([]execdriver.Mount, error) {
	var mounts []execdriver.Mount
	for _, m := range container.MountPoints {
		path, err := m.Setup()
		if err != nil {
			return nil, err
		}

		mounts = append(mounts, execdriver.Mount{
			Source:      path,
			Destination: m.Destination,
			Writable:    m.RW,
		})
	}

	mounts = sortMounts(mounts)
	return append(mounts, container.networkMounts()...), nil
}

func sortMounts(m []execdriver.Mount) []execdriver.Mount {
	sort.Sort(mounts(m))
	return m
}

type mounts []execdriver.Mount

func (m mounts) Len() int {
	return len(m)
}

func (m mounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

func (m mounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m mounts) parts(i int) int {
	return len(strings.Split(filepath.Clean(m[i].Destination), string(os.PathSeparator)))
}

// migrateVolume moves the contents of a volume created pre Docker 1.7
// to the location expected by the local driver. Steps:
// 1. Save old directory that includes old volume's config json file.
// 2. Move virtual directory with content to where the local driver expects it to be.
// 3. Remove the backup of the old volume config.
func (daemon *Daemon) migrateVolume(id, vfs string) error {
	volumeInfo := filepath.Join(daemon.root, defaultVolumesPathName, id)
	backup := filepath.Join(daemon.root, defaultVolumesPathName, id+".back")

	var err error
	if err = os.Rename(volumeInfo, backup); err != nil {
		return err
	}
	defer func() {
		// Put old configuration back in place in case one of the next steps fails.
		if err != nil {
			os.Rename(backup, volumeInfo)
		}
	}()

	if err = os.Rename(vfs, volumeInfo); err != nil {
		return err
	}

	if err = os.RemoveAll(backup); err != nil {
		logrus.Errorf("Unable to remove volume info backup directory %s: %v", backup, err)
	}

	return nil
}
