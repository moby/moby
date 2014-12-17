package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
)

func (s *TagStore) CmdManifest(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	tag := job.Getenv("tag")
	if tag == "" {
		tag = "latest"
	}

	// Resolve the Repository name from fqn to endpoint + name
	repoInfo, err := registry.ParseRepositoryInfo(name)
	if err != nil {
		return job.Error(err)
	}

	manifestBytes, err := s.newManifest(name, repoInfo.RemoteName, tag)
	if err != nil {
		return job.Error(err)
	}

	_, err = job.Stdout.Write(manifestBytes)
	if err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (s *TagStore) newManifest(localName, remoteName, tag string) ([]byte, error) {
	manifest := &registry.ManifestData{
		Name:          remoteName,
		Tag:           tag,
		SchemaVersion: 1,
	}
	localRepo, exists := s.Repositories[localName]
	if !exists {
		return nil, fmt.Errorf("Repo does not exist: %s", localName)
	}

	layerId, exists := localRepo[tag]
	if !exists {
		return nil, fmt.Errorf("Tag does not exist for %s: %s", localName, tag)
	}
	layersSeen := make(map[string]bool)

	layer, err := s.graph.Get(layerId)
	if err != nil {
		return nil, err
	}
	if layer.Config == nil {
		return nil, errors.New("Missing layer configuration")
	}
	manifest.Architecture = layer.Architecture
	manifest.FSLayers = make([]*registry.FSLayer, 0, 4)
	manifest.History = make([]*registry.ManifestHistory, 0, 4)
	var metadata runconfig.Config
	metadata = *layer.Config

	for ; layer != nil; layer, err = layer.GetParent() {
		if err != nil {
			return nil, err
		}

		if layersSeen[layer.ID] {
			break
		}
		if layer.Config != nil && metadata.Image != layer.ID {
			err = runconfig.Merge(&metadata, layer.Config)
			if err != nil {
				return nil, err
			}
		}

		archive, err := layer.TarLayer()
		if err != nil {
			return nil, err
		}

		tarSum, err := tarsum.NewTarSum(archive, true, tarsum.Version1)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(ioutil.Discard, tarSum); err != nil {
			return nil, err
		}

		tarId := tarSum.Sum(nil)
		// Save tarsum to image json

		manifest.FSLayers = append(manifest.FSLayers, &registry.FSLayer{BlobSum: tarId})

		layersSeen[layer.ID] = true
		jsonData, err := ioutil.ReadFile(path.Join(s.graph.Root, layer.ID, "json"))
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve the path for {%s}: %s", layer.ID, err)
		}
		manifest.History = append(manifest.History, &registry.ManifestHistory{V1Compatibility: string(jsonData)})
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "   ")
	if err != nil {
		return nil, err
	}

	return manifestBytes, nil
}
