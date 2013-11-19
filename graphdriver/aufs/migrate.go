package aufs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

type metadata struct {
	ID       string `json:"id"`
	ParentID string `json:"parent,omitempty"`
	Image    string `json:"Image,omitempty"`

	parent *metadata
}

func pathExists(pth string) bool {
	if _, err := os.Stat(pth); err != nil {
		return false
	}
	return true
}

// Migrate existing images and containers from docker < 0.7.x
//
// The format pre 0.7 is for docker to store the metadata and filesystem
// content in the same directory.  For the migration to work we need to move Image layer
// data from /var/lib/docker/graph/<id>/layers to the diff of the registered id.
//
// Next we need to migrate the container's rw layer to diff of the driver.  After the
// contents are migrated we need to register the image and container ids with the
// driver.
//
// For the migration we try to move the folder containing the layer files, if that
// fails because the data is currently mounted we will fallback to creating a
// symlink.
func (a *AufsDriver) Migrate(pth string, setupInit func(p string) error) error {
	if pathExists(path.Join(pth, "graph")) {
		if err := a.migrateImages(path.Join(pth, "graph")); err != nil {
			return err
		}
		return a.migrateContainers(path.Join(pth, "containers"), setupInit)
	}
	return nil
}

func (a *AufsDriver) migrateContainers(pth string, setupInit func(p string) error) error {
	fis, err := ioutil.ReadDir(pth)
	if err != nil {
		return err
	}

	for _, fi := range fis {
		if id := fi.Name(); fi.IsDir() && pathExists(path.Join(pth, id, "rw")) && !a.Exists(id) {
			if err := tryRelocate(path.Join(pth, id, "rw"), path.Join(a.rootPath(), "diff", id)); err != nil {
				return err
			}

			metadata, err := loadMetadata(path.Join(pth, id, "config.json"))
			if err != nil {
				return err
			}

			initID := fmt.Sprintf("%s-init", id)
			if err := a.Create(initID, metadata.Image); err != nil {
				return err
			}

			initPath, err := a.Get(initID)
			if err != nil {
				return err
			}
			// setup init layer
			if err := setupInit(initPath); err != nil {
				return err
			}

			if err := a.Create(id, initID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *AufsDriver) migrateImages(pth string) error {
	fis, err := ioutil.ReadDir(pth)
	if err != nil {
		return err
	}
	var (
		m       = make(map[string]*metadata)
		current *metadata
		exists  bool
	)

	for _, fi := range fis {
		if id := fi.Name(); fi.IsDir() && pathExists(path.Join(pth, id, "layer")) && !a.Exists(id) {
			if current, exists = m[id]; !exists {
				current, err = loadMetadata(path.Join(pth, id, "json"))
				if err != nil {
					return err
				}
				m[id] = current
			}
		}
	}

	for _, v := range m {
		v.parent = m[v.ParentID]
	}

	for _, v := range m {
		if err := a.migrateImage(v, pth); err != nil {
			return err
		}
	}
	return nil
}

func (a *AufsDriver) migrateImage(m *metadata, pth string) error {
	if !pathExists(path.Join(a.rootPath(), "diff", m.ID)) {
		if m.parent != nil {
			a.migrateImage(m.parent, pth)
		}
		if err := tryRelocate(path.Join(pth, m.ID, "layer"), path.Join(a.rootPath(), "diff", m.ID)); err != nil {
			return err
		}

		if err := a.Create(m.ID, m.ParentID); err != nil {
			return err
		}
	}
	return nil
}

// tryRelocate will try to rename the old path to the new pack and if
// the operation fails, it will fallback to a symlink
func tryRelocate(oldPath, newPath string) error {
	if err := os.Rename(oldPath, newPath); err != nil {
		if sErr := os.Symlink(oldPath, newPath); sErr != nil {
			return fmt.Errorf("Unable to relocate %s to %s: Rename err %s Symlink err %s", oldPath, newPath, err, sErr)
		}
	}
	return nil
}

func loadMetadata(pth string) (*metadata, error) {
	f, err := os.Open(pth)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		out = &metadata{}
		dec = json.NewDecoder(f)
	)

	if err := dec.Decode(out); err != nil {
		return nil, err
	}
	return out, nil
}
