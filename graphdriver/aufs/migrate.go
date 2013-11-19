package aufs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"
)

type imageMetadata struct {
	ID            string    `json:"id"`
	ParentID      string    `json:"parent,omitempty"`
	Created       time.Time `json:"created"`
	DockerVersion string    `json:"docker_version,omitempty"`
	Architecture  string    `json:"architecture,omitempty"`

	parent *imageMetadata
}

func pathExists(pth string) bool {
	if _, err := os.Stat(pth); err != nil {
		return false
	}
	return true
}

// Migrate existing images and containers from docker < 0.7.x
func (a *AufsDriver) Migrate(pth string) error {
	fis, err := ioutil.ReadDir(pth)
	if err != nil {
		return err
	}
	var (
		metadata = make(map[string]*imageMetadata)
		current  *imageMetadata
		exists   bool
	)

	// Load metadata
	for _, fi := range fis {
		if id := fi.Name(); fi.IsDir() && pathExists(path.Join(pth, id, "layer")) && !a.Exists(id) {
			if current, exists = metadata[id]; !exists {
				current, err = loadMetadata(pth, id)
				if err != nil {
					return err
				}
				metadata[id] = current
			}
		}
	}

	// Recreate tree
	for _, v := range metadata {
		v.parent = metadata[v.ParentID]
	}

	// Perform image migration
	for _, v := range metadata {
		if err := migrateImage(v, a, pth); err != nil {
			return err
		}
	}
	return nil
}

func migrateImage(m *imageMetadata, a *AufsDriver, pth string) error {
	if !pathExists(path.Join(a.rootPath(), "diff", m.ID)) {
		if m.parent != nil {
			migrateImage(m.parent, a, pth)
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

func loadMetadata(pth, id string) (*imageMetadata, error) {
	f, err := os.Open(path.Join(pth, id, "json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		out = &imageMetadata{}
		dec = json.NewDecoder(f)
	)

	if err := dec.Decode(out); err != nil {
		return nil, err
	}
	return out, nil
}
