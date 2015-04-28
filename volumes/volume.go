package volumes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/pkg/symlink"
)

type Volume struct {
	ID          string
	Path        string
	IsBindMount bool
	Writable    bool
	containers  map[string]struct{}
	configPath  string
	repository  *Repository
	lock        sync.Mutex
}

func (v *Volume) IsDir() (bool, error) {
	stat, err := os.Stat(v.Path)
	if err != nil {
		return false, err
	}

	return stat.IsDir(), nil
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

func (v *Volume) RemoveContainer(containerId string) {
	v.lock.Lock()
	delete(v.containers, containerId)
	v.lock.Unlock()
}

func (v *Volume) AddContainer(containerId string) {
	v.lock.Lock()
	v.containers[containerId] = struct{}{}
	v.lock.Unlock()
}

func (v *Volume) initialize() error {
	v.lock.Lock()
	defer v.lock.Unlock()

	if _, err := os.Stat(v.Path); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(v.Path, 0755); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(v.configPath, 0755); err != nil {
		return err
	}

	return v.toDisk()
}

func (v *Volume) ToDisk() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.toDisk()
}

func (v *Volume) toDisk() error {
	jsonPath, err := v.jsonPath()
	if err != nil {
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

func (v *Volume) FromDisk() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	pth, err := v.jsonPath()
	if err != nil {
		return err
	}

	jsonSource, err := os.Open(pth)
	if err != nil {
		return err
	}
	defer jsonSource.Close()

	dec := json.NewDecoder(jsonSource)

	return dec.Decode(v)
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
