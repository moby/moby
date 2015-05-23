package volumes

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/stringid"

	"github.com/docker/docker/volumes/volumedriver"
	_ "github.com/docker/docker/volumes/volumedriver/host"
	_ "github.com/docker/docker/volumes/volumedriver/vfs"
)

type Repository struct {
	configPath string
	storePath  string
	volumes    map[string]*Volume
	idIndex    map[string]*Volume
	lock       sync.Mutex
	drivers    map[string]volumedriver.Driver
}

func NewRepository(configPath string, storePath string) (*Repository, error) {
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}

	// Create dirs
	if err := os.MkdirAll(configPath, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	repo := &Repository{
		storePath:  storePath,
		configPath: configPath,
		volumes:    make(map[string]*Volume),
		idIndex:    make(map[string]*Volume),
		drivers:    make(map[string]volumedriver.Driver),
	}

	return repo, repo.restore()
}

func (r *Repository) restore() error {
	dir, err := ioutil.ReadDir(r.configPath)
	if err != nil {
		return err
	}

	for _, v := range dir {
		id := v.Name()
		vol := &Volume{
			ID:         id,
			configPath: filepath.Join(r.configPath, id),
			containers: make(map[string]struct{}),
			repository: r,
		}
		if err := vol.fromDisk(); err != nil {
			if !os.IsNotExist(err) {
				logrus.Debugf("Error restoring volume: %v", err)
				continue
			}

		}
		r.add(vol)
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
	return r.volumes[path]
}

func (r *Repository) add(volume *Volume) {
	if vol := r.get(volume.Path); vol != nil {
		return
	}
	r.volumes[volume.Path] = volume
	r.idIndex[volume.ID] = volume
}

func (r *Repository) Delete(id string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	volume := r.get(id)
	if volume == nil {
		return fmt.Errorf("Volume %s does not exist", id)
	}

	containers := volume.Containers()
	if len(containers) > 0 {
		return fmt.Errorf("Volume %s is being used and cannot be removed: used by containers %s", volume.Path, containers)
	}

	if err := os.RemoveAll(volume.configPath); err != nil {
		return err
	}

	if err := volume.remove(); err != nil {
		return err
	}

	delete(r.volumes, volume.Path)
	delete(r.idIndex, volume.ID)
	return nil
}

func (r *Repository) FindOrCreateVolume(path, driver string) (*Volume, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if v := r.get(path); v != nil {
		logrus.Debugf("found existing volume for: %s %v", driver)
		if !v.exists() {
			// Recreates the underlying volume: #10146
			logrus.Debugf("recreating volume %s", v.Path)
			if err := v.create(); err != nil {
				return nil, err
			}
		}
		return v, nil
	}

	v, err := r.newVolume(path, driver)
	if err != nil {
		return nil, err
	}

	if err := v.toDisk(); err != nil {
		v.remove()
		return nil, err
	}
	r.add(v)
	return v, nil
}

func (r *Repository) newVolume(path, driver string) (*Volume, error) {
	id, err := r.generateId()
	if err != nil {
		return nil, err
	}

	if path == "" && driver == "" {
		driver = "vfs"
	}

	if driver == "" {
		driver = r.getDriverNameFromPath(path)
		if driver == "" {
			driver = "vfs"
		}
	}

	d, err := r.getDriver(driver)
	if err != nil {
		return nil, err
	}

	if driver != "host" {
		path = filepath.Join(r.storePath, driver, id)
	}

	if err := d.Create(path); err != nil {
		return nil, err
	}

	configPath := filepath.Join(r.configPath, id)
	return &Volume{
		ID:         id,
		Path:       path,
		driver:     d,
		containers: make(map[string]struct{}),
		configPath: configPath}, nil
}

func (r *Repository) generateId() (string, error) {
	for i := 0; i < 5; i++ {
		id := stringid.GenerateRandomID()
		if _, exists := r.idIndex[id]; !exists {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique volume ID")
}

func (r *Repository) getDriver(name string) (volumedriver.Driver, error) {
	if d, exists := r.drivers[name]; exists {
		return d, nil
	}

	d, err := volumedriver.NewDriver(name)
	if err != nil {
		return nil, err
	}
	r.drivers[name] = d
	return d, nil
}

func (r *Repository) getDriverNameFromPath(path string) string {
	path = filepath.Clean(path)

	// strip away the id
	path = filepath.Dir(path)

	// Check for old graphdriver vfs path
	// Using a new storePath which is inside of the path handed to us by the daemon, so need to strip that for this check
	if path == filepath.Join(filepath.Dir(r.storePath), "vfs", "dir") {
		return "vfs"
	}

	name := filepath.Base(path)
	if _, err := r.getDriver(name); err == nil && path == filepath.Join(r.storePath, name) {
		return name
	}

	return "host"
}
