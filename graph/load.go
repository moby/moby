// +build linux

package graph

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/utils"
)

// Loads a set of images into the repository. This is the complementary of ImageExport.
// The input stream is an uncompressed tar ball containing images and metadata.
func (s *TagStore) CmdLoad(job *engine.Job) engine.Status {
	tmpImageDir, err := ioutil.TempDir("", "docker-import-")
	if err != nil {
		return job.Error(err)
	}
	defer os.RemoveAll(tmpImageDir)

	var (
		repoDir = path.Join(tmpImageDir, "repo")
	)

	if err := os.Mkdir(repoDir, os.ModeDir); err != nil {
		return job.Error(err)
	}
	images, err := s.graph.Map()
	if err != nil {
		return job.Error(err)
	}
	excludes := make([]string, len(images))
	i := 0
	for k := range images {
		excludes[i] = k
		i++
	}
	if err := chrootarchive.Untar(job.Stdin, repoDir, &archive.TarOptions{ExcludePatterns: excludes}); err != nil {
		return job.Error(err)
	}

	dirs, err := ioutil.ReadDir(repoDir)
	if err != nil {
		return job.Error(err)
	}

	for _, d := range dirs {
		if d.IsDir() {
			if err := s.recursiveLoad(job.Eng, d.Name(), tmpImageDir); err != nil {
				return job.Error(err)
			}
		}
	}

	repositoriesJson, err := ioutil.ReadFile(path.Join(tmpImageDir, "repo", "repositories"))
	if err == nil {
		repositories := map[string]Repository{}
		if err := json.Unmarshal(repositoriesJson, &repositories); err != nil {
			return job.Error(err)
		}

		for imageName, tagMap := range repositories {
			for tag, address := range tagMap {
				if err := s.Set(imageName, tag, address, true); err != nil {
					return job.Error(err)
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (s *TagStore) recursiveLoad(eng *engine.Engine, address, tmpImageDir string) error {
	if err := eng.Job("image_get", address).Run(); err != nil {
		log.Debugf("Loading %s", address)

		imageJson, err := ioutil.ReadFile(path.Join(tmpImageDir, "repo", address, "json"))
		if err != nil {
			log.Debugf("Error reading json", err)
			return err
		}

		layer, err := os.Open(path.Join(tmpImageDir, "repo", address, "layer.tar"))
		if err != nil {
			log.Debugf("Error reading embedded tar", err)
			return err
		}
		img, err := image.NewImgJSON(imageJson)
		if err != nil {
			log.Debugf("Error unmarshalling json", err)
			return err
		}
		if err := utils.ValidateID(img.ID); err != nil {
			log.Debugf("Error validating ID: %s", err)
			return err
		}

		// ensure no two downloads of the same layer happen at the same time
		if c, err := s.poolAdd("pull", "layer:"+img.ID); err != nil {
			if c != nil {
				log.Debugf("Image (id: %s) load is already running, waiting: %v", img.ID, err)
				<-c
				return nil
			}

			return err
		}

		defer s.poolRemove("pull", "layer:"+img.ID)

		if img.Parent != "" {
			if !s.graph.Exists(img.Parent) {
				if err := s.recursiveLoad(eng, img.Parent, tmpImageDir); err != nil {
					return err
				}
			}
		}
		if err := s.graph.Register(img, layer); err != nil {
			return err
		}
	}
	log.Debugf("Completed processing %s", address)

	return nil
}
