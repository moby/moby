package v1

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/image"
	imagev1 "github.com/docker/docker/image/v1"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/tag"
)

type graphIDRegistrar interface {
	RegisterByGraphID(string, layer.ChainID, string) (layer.Layer, error)
	Release(layer.Layer) ([]layer.Metadata, error)
}

type graphIDMounter interface {
	MountByGraphID(string, string, layer.ChainID) (layer.RWLayer, error)
	Unmount(string) error
}

const (
	graphDirName                 = "graph"
	tarDataFileName              = "tar-data.json.gz"
	migrationFileName            = ".migration-v1-images.json"
	migrationTagsFileName        = ".migration-v1-tags"
	containersDirName            = "containers"
	configFileNameLegacy         = "config.json"
	configFileName               = "config.v2.json"
	repositoriesFilePrefixLegacy = "repositories-"
)

var (
	errUnsupported = errors.New("migration is not supported")
)

// Migrate takes an old graph directory and transforms the metadata into the
// new format.
func Migrate(root, driverName string, ls layer.Store, is image.Store, ts tag.Store, ms metadata.Store) error {
	mappings := make(map[string]image.ID)

	if registrar, ok := ls.(graphIDRegistrar); !ok {
		return errUnsupported
	} else if err := migrateImages(root, registrar, is, ms, mappings); err != nil {
		return err
	}

	if mounter, ok := ls.(graphIDMounter); !ok {
		return errUnsupported
	} else if err := migrateContainers(root, mounter, is, mappings); err != nil {
		return err
	}

	if err := migrateTags(root, driverName, ts, mappings); err != nil {
		return err
	}

	return nil
}

func migrateImages(root string, ls graphIDRegistrar, is image.Store, ms metadata.Store, mappings map[string]image.ID) error {
	graphDir := filepath.Join(root, graphDirName)
	if _, err := os.Lstat(graphDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	mfile := filepath.Join(root, migrationFileName)
	f, err := os.Open(mfile)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		err := json.NewDecoder(f).Decode(&mappings)
		if err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	dir, err := ioutil.ReadDir(graphDir)
	if err != nil {
		return err
	}
	for _, v := range dir {
		v1ID := v.Name()
		if err := imagev1.ValidateID(v1ID); err != nil {
			continue
		}
		if _, exists := mappings[v1ID]; exists {
			continue
		}
		if err := migrateImage(v1ID, root, ls, is, ms, mappings); err != nil {
			continue
		}
	}

	f, err = os.OpenFile(mfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(mappings); err != nil {
		return err
	}

	return nil
}

func migrateContainers(root string, ls graphIDMounter, is image.Store, imageMappings map[string]image.ID) error {
	containersDir := filepath.Join(root, containersDirName)
	dir, err := ioutil.ReadDir(containersDir)
	if err != nil {
		return err
	}
	for _, v := range dir {
		id := v.Name()

		if _, err := os.Stat(filepath.Join(containersDir, id, configFileName)); err == nil {
			continue
		}

		containerJSON, err := ioutil.ReadFile(filepath.Join(containersDir, id, configFileNameLegacy))
		if err != nil {
			return err
		}

		var c map[string]*json.RawMessage
		if err := json.Unmarshal(containerJSON, &c); err != nil {
			return err
		}

		imageStrJSON, ok := c["Image"]
		if !ok {
			return fmt.Errorf("invalid container configuration for %v", id)
		}

		var image string
		if err := json.Unmarshal([]byte(*imageStrJSON), &image); err != nil {
			return err
		}
		imageID, ok := imageMappings[image]
		if !ok {
			logrus.Errorf("image not migrated %v", imageID) // non-fatal error
			continue
		}

		c["Image"] = rawJSON(imageID)

		containerJSON, err = json.Marshal(c)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(filepath.Join(containersDir, id, configFileName), containerJSON, 0600); err != nil {
			return err
		}

		img, err := is.Get(imageID)
		if err != nil {
			return err
		}

		_, err = ls.MountByGraphID(id, id, img.RootFS.ChainID())
		if err != nil {
			return err
		}

		err = ls.Unmount(id)
		if err != nil {
			return err
		}

		logrus.Infof("migrated container %s to point to %s", id, imageID)

	}
	return nil
}

type tagAdder interface {
	AddTag(ref reference.Named, id image.ID, force bool) error
	AddDigest(ref reference.Canonical, id image.ID, force bool) error
}

func migrateTags(root, driverName string, ts tagAdder, mappings map[string]image.ID) error {
	migrationFile := filepath.Join(root, migrationTagsFileName)
	if _, err := os.Lstat(migrationFile); !os.IsNotExist(err) {
		return err
	}

	type repositories struct {
		Repositories map[string]map[string]string
	}

	var repos repositories

	f, err := os.Open(filepath.Join(root, repositoriesFilePrefixLegacy+driverName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&repos); err != nil {
		return err
	}

	for name, repo := range repos.Repositories {
		for tag, id := range repo {
			if strongID, exists := mappings[id]; exists {
				ref, err := reference.WithName(name)
				if err != nil {
					logrus.Errorf("migrate tags: invalid name %q, %q", name, err)
					continue
				}
				if dgst, err := digest.ParseDigest(tag); err == nil {
					canonical, err := reference.WithDigest(ref, dgst)
					if err != nil {
						logrus.Errorf("migrate tags: invalid digest %q, %q", dgst, err)
						continue
					}
					if err := ts.AddDigest(canonical, strongID, false); err != nil {
						logrus.Errorf("can't migrate digest %q for %q, err: %q", ref.String(), strongID, err)
					}
				} else {
					tagRef, err := reference.WithTag(ref, tag)
					if err != nil {
						logrus.Errorf("migrate tags: invalid tag %q, %q", tag, err)
						continue
					}
					if err := ts.AddTag(tagRef, strongID, false); err != nil {
						logrus.Errorf("can't migrate tag %q for %q, err: %q", ref.String(), strongID, err)
					}
				}
				logrus.Infof("migrated tag %s:%s to point to %s", name, tag, strongID)
			}
		}
	}

	mf, err := os.Create(migrationFile)
	if err != nil {
		return err
	}
	mf.Close()

	return nil
}

func migrateImage(id, root string, ls graphIDRegistrar, is image.Store, ms metadata.Store, mappings map[string]image.ID) (err error) {
	defer func() {
		if err != nil {
			logrus.Errorf("migration failed for %v, err: %v", id, err)
		}
	}()

	jsonFile := filepath.Join(root, graphDirName, id, "json")
	imageJSON, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		return err
	}
	var parent struct {
		Parent   string
		ParentID digest.Digest `json:"parent_id"`
	}
	if err := json.Unmarshal(imageJSON, &parent); err != nil {
		return err
	}
	if parent.Parent == "" && parent.ParentID != "" { // v1.9
		parent.Parent = parent.ParentID.Hex()
	}
	// compatibilityID for parent
	parentCompatibilityID, err := ioutil.ReadFile(filepath.Join(root, graphDirName, id, "parent"))
	if err == nil && len(parentCompatibilityID) > 0 {
		parent.Parent = string(parentCompatibilityID)
	}

	var parentID image.ID
	if parent.Parent != "" {
		var exists bool
		if parentID, exists = mappings[parent.Parent]; !exists {
			if err := migrateImage(parent.Parent, root, ls, is, ms, mappings); err != nil {
				// todo: fail or allow broken chains?
				return err
			}
			parentID = mappings[parent.Parent]
		}
	}

	rootFS := image.NewRootFS()
	var history []image.History

	if parentID != "" {
		parentImg, err := is.Get(parentID)
		if err != nil {
			return err
		}

		rootFS = parentImg.RootFS
		history = parentImg.History
	}

	layer, err := ls.RegisterByGraphID(id, rootFS.ChainID(), filepath.Join(filepath.Join(root, graphDirName, id, tarDataFileName)))
	if err != nil {
		return err
	}
	logrus.Infof("migrated layer %s to %s", id, layer.DiffID())

	h, err := imagev1.HistoryFromConfig(imageJSON, false)
	if err != nil {
		return err
	}
	history = append(history, h)

	rootFS.Append(layer.DiffID())

	config, err := imagev1.MakeConfigFromV1Config(imageJSON, rootFS, history)
	if err != nil {
		return err
	}
	strongID, err := is.Create(config)
	if err != nil {
		return err
	}
	logrus.Infof("migrated image %s to %s", id, strongID)

	if parentID != "" {
		if err := is.SetParent(strongID, parentID); err != nil {
			return err
		}
	}

	checksum, err := ioutil.ReadFile(filepath.Join(root, graphDirName, id, "checksum"))
	if err == nil { // best effort
		dgst, err := digest.ParseDigest(string(checksum))
		if err == nil {
			blobSumService := metadata.NewBlobSumService(ms)
			blobSumService.Add(layer.DiffID(), dgst)
		}
	}
	_, err = ls.Release(layer)
	if err != nil {
		return err
	}

	mappings[id] = strongID
	return
}

func rawJSON(value interface{}) *json.RawMessage {
	jsonval, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return (*json.RawMessage)(&jsonval)
}
