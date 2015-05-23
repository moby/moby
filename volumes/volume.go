package volumes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/volumes/volumedriver"
)

type Volume struct {
	ID         string // ID assigned by docker
	containers map[string]struct{}
	configPath string
	Path       string
	Writable   bool // TODO: Maybe this isn't needed
	DriverName string
	driver     volumedriver.Driver
	repository *Repository
	lock       sync.Mutex
}

func (v *Volume) create() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.driver.Create(v.Path)
}

func (v *Volume) remove() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	if len(v.containers) > 0 {
		return fmt.Errorf("volume is in use, cannot remove")
	}
	return v.driver.Remove(v.Path)
}

func (v *Volume) Link(containerID string) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	if err := v.driver.Link(v.Path, containerID); err != nil {
		return err
	}
	v.containers[containerID] = struct{}{}
	return nil
}

func (v *Volume) Unlink(containerID string) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	if err := v.driver.Unlink(v.Path, containerID); err != nil {
		return err
	}
	delete(v.containers, containerID)
	return nil
}

func (v *Volume) exists() bool {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.driver.Exists(v.Path)
}

func (v *Volume) Containers() []string {
	v.lock.Lock()

	var containers []string
	for c := range v.containers {
		containers = append(containers, c)
	}

	v.lock.Unlock()
	return containers
}

func (v *Volume) toDisk() error {
	jsonPath, err := v.jsonPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(v.configPath, 0750); err != nil {
		return err
	}

	f, err := os.OpenFile(jsonPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(v); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (v *Volume) fromDisk() error {
	v.lock.Lock()
	defer v.lock.Unlock()

	pth, err := v.jsonPath()
	if err != nil {
		return err
	}

	data, err := ioutil.ReadFile(pth)
	if err != nil {
		return err
	}

	type cfg struct {
		DriverName string
		// This is from older docker versions before volume drivers
		// We'll use this to determine the type of volume
		IsBindMount bool
	}

	var config *cfg
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("error getting driver from volume config json: %v", err)
	}
	if config.DriverName == "" {
		config.DriverName = "vfs"
		if config.IsBindMount {
			config.DriverName = "host"
		}
	}

	driver, err := v.repository.getDriver(config.DriverName)
	if err != nil {
		return err
	}
	v.driver = driver
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("error reading volume config json: %v", err)
	}

	return nil
}

func (v *Volume) jsonPath() (string, error) {
	return v.GetRootResourcePath("config.json")
}

// Evalutes `path` in the scope of the volume's root path, with proper path
// sanitisation. Symlinks are all scoped to the root of the volume, as
// though the volume's root was `/`.
//
// The volume's root path is the host-facing path of the root of the volume's
// mountpoint inside a container.
//
// NOTE: The returned path is *only* safely scoped inside the volume's root
//       if no component of the returned path changes (such as a component
//       symlinking to a different path) between using this method and using the
//       path. See symlink.FollowSymlinkInScope for more details.
func (v *Volume) GetResourcePath(path string) (string, error) {
	cleanPath := filepath.Join("/", path)
	return symlink.FollowSymlinkInScope(filepath.Join(v.Path, cleanPath), v.Path)
}

// Evalutes `path` in the scope of the volume's config path, with proper path
// sanitisation. Symlinks are all scoped to the root of the config path, as
// though the config path was `/`.
//
// The config path of a volume is not exposed to the container and is just used
// to store volume configuration options and other internal information. If in
// doubt, you probably want to just use v.GetResourcePath.
//
// NOTE: The returned path is *only* safely scoped inside the volume's config
//       path if no component of the returned path changes (such as a component
//       symlinking to a different path) between using this method and using the
//       path. See symlink.FollowSymlinkInScope for more details.
func (v *Volume) GetRootResourcePath(path string) (string, error) {
	cleanPath := filepath.Join("/", path)
	return symlink.FollowSymlinkInScope(filepath.Join(v.configPath, cleanPath), v.configPath)
}
