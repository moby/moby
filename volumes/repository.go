package volumes

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	log "github.com/Sirupsen/logrus"
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

func (r *Repository) newVolume(path string, writable bool) (*Volume, error) {
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
	path = filepath.Clean(path)

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}

	v := &Volume{
		ID:          id,
		Path:        path,
		repository:  r,
		Writable:    writable,
		containers:  make(map[string]struct{}),
		configPath:  r.configPath + "/" + id,
		IsBindMount: isBindMount,
	}

	if err := v.initialize(); err != nil {
		return nil, err
	}

	return v, r.add(v)
}

func (r *Repository) restore() error {
	dir, err := ioutil.ReadDir(r.configPath)
	if err != nil {
		return err
	}

	for _, v := range dir {
		id := v.Name()
		path, err := r.driver.Get(id, "")
		if err != nil {
			log.Debugf("Could not find volume for %s: %v", id, err)
			continue
		}
		vol := &Volume{
			ID:         id,
			configPath: r.configPath + "/" + id,
			containers: make(map[string]struct{}),
			Path:       path,
		}
		if err := vol.FromDisk(); err != nil {
			if !os.IsNotExist(err) {
				log.Debugf("Error restoring volume: %v", err)
				continue
			}
			if err := vol.initialize(); err != nil {
				log.Debugf("%s", err)
				continue
			}
		}
		if err := r.add(vol); err != nil {
			log.Debugf("Error restoring volume: %v", err)
		}
	}
	return nil
}

func (r *Repository) Get(path string) *Volume {
	r.lock.Lock()
	vol := r.get(path)
	r.lock.Unlock()
	return vol
}

func (r *Repository) get(path string) *Volume {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil
	}
	return r.volumes[filepath.Clean(path)]
}

func (r *Repository) Add(volume *Volume) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.add(volume)
}

func (r *Repository) add(volume *Volume) error {
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
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	volume := r.get(filepath.Clean(path))
	if volume == nil {
		return fmt.Errorf("Volume %s does not exist", path)
	}

	containers := volume.Containers()
	if len(containers) > 0 {
		return fmt.Errorf("Volume %s is being used and cannot be removed: used by containers %s", volume.Path, containers)
	}

	if err := os.RemoveAll(volume.configPath); err != nil {
		return err
	}

	if volume.IsBindMount {
		return nil
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
		return "", fmt.Errorf("Driver %s failed to get volume rootfs %s: %v", r.driver, id, err)
	}

	return path, nil
}

func (r *Repository) FindOrCreateVolume(path string, writable bool) (*Volume, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if path == "" {
		return r.newVolume(path, writable)
	}

	if v := r.get(path); v != nil {
		return v, nil
	}

	return r.newVolume(path, writable)
}
