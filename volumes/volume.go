package volumes

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/docker/docker/pkg/archive"
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

func (v *Volume) Export(resource, name string) (io.ReadCloser, error) {
	if v.IsBindMount && filepath.Base(resource) == name {
		name = ""
	}

	basePath, err := v.getResourcePath(resource)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(basePath)
	if err != nil {
		return nil, err
	}
	var filter []string
	if !stat.IsDir() {
		d, f := path.Split(basePath)
		basePath = d
		filter = []string{f}
	} else {
		filter = []string{path.Base(basePath)}
		basePath = path.Dir(basePath)
	}
	return archive.TarWithOptions(basePath, &archive.TarOptions{
		Compression: archive.Uncompressed,
		Name:        name,
		Includes:    filter,
	})
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

func (v *Volume) createIfNotExist() error {
	if stat, err := os.Stat(v.Path); err != nil && os.IsNotExist(err) {
		if stat.IsDir() {
			os.MkdirAll(v.Path, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(v.Path), 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(v.Path, os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

func (v *Volume) initialize() error {
	v.lock.Lock()
	defer v.lock.Unlock()

	if err := v.createIfNotExist(); err != nil {
		return err
	}

	if err := os.MkdirAll(v.configPath, 0755); err != nil {
		return err
	}
	jsonPath, err := v.jsonPath()
	if err != nil {
		return err
	}
	f, err := os.Create(jsonPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return v.toDisk()
}

func (v *Volume) ToDisk() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.toDisk()
}
func (v *Volume) toDisk() error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	pth, err := v.jsonPath()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(pth, data, 0666)
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
	return v.getRootResourcePath("config.json")
}
func (v *Volume) getRootResourcePath(path string) (string, error) {
	cleanPath := filepath.Join("/", path)
	return symlink.FollowSymlinkInScope(filepath.Join(v.configPath, cleanPath), v.configPath)
}

func (v *Volume) getResourcePath(path string) (string, error) {
	cleanPath := filepath.Join("/", path)
	return symlink.FollowSymlinkInScope(filepath.Join(v.Path, cleanPath), v.Path)
}
