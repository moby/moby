package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

const (
	testManifestImageName    = "testapp"
	testManifestImageID      = "d821b739e8834ec89ac4469266c3d11515da88fdcbcbdddcbcddb636f54fdde9"
	testManifestImageIDShort = "d821b739e883"
	testManifestTag          = "manifesttest"
)

func (s *TagStore) newManifest(localName, remoteName, tag string) ([]byte, error) {
	manifest := &registry.ManifestData{
		Name:          remoteName,
		Tag:           tag,
		SchemaVersion: 1,
	}
	localRepo, err := s.Get(localName)
	if err != nil {
		return nil, err
	}
	if localRepo == nil {
		return nil, fmt.Errorf("Repo does not exist: %s", localName)
	}

	// Get the top-most layer id which the tag points to
	layerId, exists := localRepo[tag]
	if !exists {
		return nil, fmt.Errorf("Tag does not exist for %s: %s", localName, tag)
	}
	layersSeen := make(map[string]bool)

	layer, err := s.graph.Get(layerId)
	if err != nil {
		return nil, err
	}
	manifest.Architecture = layer.Architecture
	manifest.FSLayers = make([]*registry.FSLayer, 0, 4)
	manifest.History = make([]*registry.ManifestHistory, 0, 4)
	var metadata runconfig.Config
	if layer.Config != nil {
		metadata = *layer.Config
	}

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

		checksum, err := layer.GetCheckSum(s.graph.ImageRoot(layer.ID))
		if err != nil {
			return nil, fmt.Errorf("Error getting image checksum: %s", err)
		}
		if tarsum.VersionLabelForChecksum(checksum) != tarsum.Version1.String() {
			archive, err := layer.TarLayer()
			if err != nil {
				return nil, err
			}

			defer archive.Close()

			tarSum, err := tarsum.NewTarSum(archive, true, tarsum.Version1)
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(ioutil.Discard, tarSum); err != nil {
				return nil, err
			}

			checksum = tarSum.Sum(nil)

			// Save checksum value
			if err := layer.SaveCheckSum(s.graph.ImageRoot(layer.ID), checksum); err != nil {
				return nil, err
			}
		}

		jsonData, err := layer.RawJson()
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve the path for {%s}: %s", layer.ID, err)
		}

		manifest.FSLayers = append(manifest.FSLayers, &registry.FSLayer{BlobSum: checksum})

		layersSeen[layer.ID] = true

		manifest.History = append(manifest.History, &registry.ManifestHistory{V1Compatibility: string(jsonData)})
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "   ")
	if err != nil {
		return nil, err
	}

	return manifestBytes, nil
}

func TestManifestTarsumCache(t *testing.T) {
	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	store := mkTestTagStore(tmp, t)
	defer store.graph.driver.Cleanup()

	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	img := &image.Image{ID: testManifestImageID}
	if err := store.graph.Register(img, archive); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(testManifestImageName, testManifestTag, testManifestImageID, false); err != nil {
		t.Fatal(err)
	}

	if cs, err := img.GetCheckSum(store.graph.ImageRoot(testManifestImageID)); err != nil {
		t.Fatal(err)
	} else if cs != "" {
		t.Fatalf("Non-empty checksum file after register")
	}

	// Generate manifest
	payload, err := store.newManifest(testManifestImageName, testManifestImageName, testManifestTag)
	if err != nil {
		t.Fatal(err)
	}

	manifestChecksum, err := img.GetCheckSum(store.graph.ImageRoot(testManifestImageID))
	if err != nil {
		t.Fatal(err)
	}

	var manifest registry.ManifestData
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("error unmarshalling manifest: %s", err)
	}

	if len(manifest.FSLayers) != 1 {
		t.Fatalf("Unexpected number of layers, expecting 1: %d", len(manifest.FSLayers))
	}

	if manifest.FSLayers[0].BlobSum != manifestChecksum {
		t.Fatalf("Unexpected blob sum, expecting %q, got %q", manifestChecksum, manifest.FSLayers[0].BlobSum)
	}

	if len(manifest.History) != 1 {
		t.Fatalf("Unexpected number of layer history, expecting 1: %d", len(manifest.History))
	}

	v1compat, err := img.RawJson()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.History[0].V1Compatibility != string(v1compat) {
		t.Fatalf("Unexpected json value\nExpected:\n%s\nActual:\n%s", v1compat, manifest.History[0].V1Compatibility)
	}
}
