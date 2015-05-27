package local

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/volume"
)

func New(rootDirectory string) (*Root, error) {
	if err := os.MkdirAll(rootDirectory, 0700); err != nil {
		return nil, err
	}
	r := &Root{
		path:    rootDirectory,
		volumes: make(map[string]*Volume),
	}
	dirs, err := ioutil.ReadDir(rootDirectory)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		name := filepath.Base(d.Name())
		r.volumes[name] = &Volume{
			driverName: r.Name(),
			name:       name,
			path:       filepath.Join(rootDirectory, name),
		}
	}
	return r, nil
}

type Root struct {
	m       sync.Mutex
	path    string
	volumes map[string]*Volume
}

func (r *Root) Name() string {
	return "local"
}

func (r *Root) Create(name string) (volume.Volume, error) {
	r.m.Lock()
	defer r.m.Unlock()
	v, exists := r.volumes[name]
	if !exists {
		path := filepath.Join(r.path, name)
		if err := os.Mkdir(path, 0755); err != nil {
			if os.IsExist(err) {
				return nil, fmt.Errorf("volume already exists under %s", path)
			}
			return nil, err
		}
		v = &Volume{
			driverName: r.Name(),
			name:       name,
			path:       path,
		}
		r.volumes[name] = v
	}
	v.use()
	return v, nil
}

func (r *Root) Remove(v volume.Volume) error {
	r.m.Lock()
	defer r.m.Unlock()
	lv, ok := v.(*Volume)
	if !ok {
		return errors.New("unknown volume type")
	}
	lv.release()
	if lv.usedCount == 0 {
		delete(r.volumes, lv.name)
		return os.RemoveAll(lv.path)
	}
	return nil
}

type Volume struct {
	m         sync.Mutex
	usedCount int
	// unique name of the volume
	name string
	// path is the path on the host where the data lives
	path string
	// driverName is the name of the driver that created the volume.
	driverName string
}

func (v *Volume) Name() string {
	return v.name
}

func (v *Volume) DriverName() string {
	return v.driverName
}

func (v *Volume) Path() string {
	return v.path
}

func (v *Volume) Mount() (string, error) {
	return v.path, nil
}

func (v *Volume) Unmount() error {
	return nil
}

func (v *Volume) use() {
	v.m.Lock()
	v.usedCount++
	v.m.Unlock()
}

func (v *Volume) release() {
	v.m.Lock()
	v.usedCount--
	v.m.Unlock()
}
