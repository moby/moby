package volumes

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/utils"
)

type Repository struct {
	configPath string
	driver     graphdriver.Driver
	volumes    map[string]*Volume
	lock       sync.Mutex
}

func NewRepository(configPath string, driver graphdriver.Driver) (*Repository, error) {
	abspath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}

	// Create the config path
	if err := os.MkdirAll(abspath, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	repo := &Repository{
		driver:     driver,
		configPath: abspath,
		volumes:    make(map[string]*Volume),
	}

	return repo, repo.restore()
}

func (r *Repository) NewVolume(path string, writable bool) (*Volume, error) {
	var (
		isBindMount bool
		err         error
		id          = utils.GenerateRandomID()
	)
	if path != "" {
		isBindMount = true
	}

	if path == "" {
		path, err = r.createNewVolumePath(id)
		if err != nil {
			return nil, err
		}
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}

	v := &Volume{
		ID:          id,
		Path:        path,
		repository:  r,
		Writable:    writable,
		Containers:  make(map[string]struct{}),
		configPath:  r.configPath + "/" + id,
		IsBindMount: isBindMount,
	}

	if err := v.initialize(); err != nil {
		return nil, err
	}
	if err := r.Add(v); err != nil {
		return nil, err
	}
	return v, nil
}

func (r *Repository) restore() error {
	dir, err := ioutil.ReadDir(r.configPath)
	if err != nil {
		return err
	}

	var ids []string
	for _, v := range dir {
		id := v.Name()
		if r.driver.Exists(id) {
			ids = append(ids, id)
		}
	}
	return nil
}

func (r *Repository) Get(path string) *Volume {
	r.lock.Lock()
	vol := r.volumes[path]
	r.lock.Unlock()
	return vol
}

func (r *Repository) get(path string) *Volume {
	return r.volumes[path]
}

func (r *Repository) Add(volume *Volume) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if vol := r.get(volume.Path); vol != nil {
		return fmt.Errorf("Volume exists: %s", volume.ID)
	}
	r.volumes[volume.Path] = volume
	return nil
}

func (r *Repository) Remove(volume *Volume) {
	r.lock.Lock()
	r.remove(volume)
	r.lock.Unlock()
}

func (r *Repository) remove(volume *Volume) {
	delete(r.volumes, volume.Path)
}

func (r *Repository) Delete(path string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	volume := r.get(path)
	if volume == nil {
		return fmt.Errorf("Volume %s does not exist", path)
	}

	if volume.IsBindMount {
		return fmt.Errorf("Volume %s is a bind-mount and cannot be removed", volume.Path)
	}
	if len(volume.Containers) > 0 {
		return fmt.Errorf("Volume %s is being used and cannot be removed: used by containers %s", volume.Path, volume.Containers)
	}

	if err := os.RemoveAll(volume.configPath); err != nil {
		return err
	}

	if err := r.driver.Remove(volume.ID); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	r.remove(volume)
	return nil
}

func (r *Repository) createNewVolumePath(id string) (string, error) {
	if err := r.driver.Create(id, ""); err != nil {
		return "", err
	}

	path, err := r.driver.Get(id, "")
	if err != nil {
		return "", fmt.Errorf("Driver %s failed to get volume rootfs %s: %s", r.driver, id, err)
	}

	return path, nil
}
