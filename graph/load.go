package graph

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/docker/docker/archive"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/log"
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
		repoTarFile = path.Join(tmpImageDir, "repo.tar")
		repoDir     = path.Join(tmpImageDir, "repo")
	)

	tarFile, err := os.Create(repoTarFile)
	if err != nil {
		return job.Error(err)
	}
	if _, err := io.Copy(tarFile, job.Stdin); err != nil {
		return job.Error(err)
	}
	tarFile.Close()

	repoFile, err := os.Open(repoTarFile)
	if err != nil {
		return job.Error(err)
	}
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
	if err := archive.Untar(repoFile, repoDir, &archive.TarOptions{Excludes: excludes}); err != nil {
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
		if img.Parent != "" {
			if !s.graph.Exists(img.Parent) {
				if err := s.recursiveLoad(eng, img.Parent, tmpImageDir); err != nil {
					return err
				}
			}
		}
		if err := s.graph.Register(imageJson, layer, img); err != nil {
			return err
		}
	}
	log.Debugf("Completed processing %s", address)

	return nil
}
