package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
	"github.com/docker/libtrust"
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

	for ; layer != nil; layer, err = s.graph.GetParent(layer) {
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

		dgst, err := s.graph.GetDigest(layer.ID)
		if err == ErrDigestNotSet {
			archive, err := s.graph.TarLayer(layer)
			if err != nil {
				return nil, err
			}

			defer archive.Close()

			dgst, err = digest.FromReader(archive)
			if err != nil {
				return nil, err
			}

			// Save checksum value
			if err := s.graph.SetDigest(layer.ID, dgst); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, fmt.Errorf("Error getting image checksum: %s", err)
		}

		jsonData, err := s.graph.RawJSON(layer.ID)
		if err != nil {
			return nil, fmt.Errorf("Cannot retrieve the path for {%s}: %s", layer.ID, err)
		}

		manifest.FSLayers = append(manifest.FSLayers, &registry.FSLayer{BlobSum: dgst.String()})

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
	img := &Image{ID: testManifestImageID}
	if err := store.graph.Register(img, archive); err != nil {
		t.Fatal(err)
	}
	if err := store.Tag(testManifestImageName, testManifestTag, testManifestImageID, false); err != nil {
		t.Fatal(err)
	}

	if _, err := store.graph.GetDigest(testManifestImageID); err == nil {
		t.Fatalf("Non-empty checksum file after register")
	} else if err != ErrDigestNotSet {
		t.Fatal(err)
	}

	// Generate manifest
	payload, err := store.newManifest(testManifestImageName, testManifestImageName, testManifestTag)
	if err != nil {
		t.Fatal(err)
	}

	manifestChecksum, err := store.graph.GetDigest(testManifestImageID)
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

	if manifest.FSLayers[0].BlobSum != manifestChecksum.String() {
		t.Fatalf("Unexpected blob sum, expecting %q, got %q", manifestChecksum, manifest.FSLayers[0].BlobSum)
	}

	if len(manifest.History) != 1 {
		t.Fatalf("Unexpected number of layer history, expecting 1: %d", len(manifest.History))
	}

	v1compat, err := store.graph.RawJSON(img.ID)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.History[0].V1Compatibility != string(v1compat) {
		t.Fatalf("Unexpected json value\nExpected:\n%s\nActual:\n%s", v1compat, manifest.History[0].V1Compatibility)
	}
}

// TestManifestDigestCheck ensures that loadManifest properly verifies the
// remote and local digest.
func TestManifestDigestCheck(t *testing.T) {
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
	img := &Image{ID: testManifestImageID}
	if err := store.graph.Register(img, archive); err != nil {
		t.Fatal(err)
	}
	if err := store.Tag(testManifestImageName, testManifestTag, testManifestImageID, false); err != nil {
		t.Fatal(err)
	}

	if _, err := store.graph.GetDigest(testManifestImageID); err == nil {
		t.Fatalf("Non-empty checksum file after register")
	} else if err != ErrDigestNotSet {
		t.Fatal(err)
	}

	// Generate manifest
	payload, err := store.newManifest(testManifestImageName, testManifestImageName, testManifestTag)
	if err != nil {
		t.Fatalf("unexpected error generating test manifest: %v", err)
	}

	pk, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		t.Fatalf("unexpected error generating private key: %v", err)
	}

	sig, err := libtrust.NewJSONSignature(payload)
	if err != nil {
		t.Fatalf("error creating signature: %v", err)
	}

	if err := sig.Sign(pk); err != nil {
		t.Fatalf("error signing manifest bytes: %v", err)
	}

	signedBytes, err := sig.PrettySignature("signatures")
	if err != nil {
		t.Fatalf("error getting signed bytes: %v", err)
	}

	dgst, err := digest.FromBytes(payload)
	if err != nil {
		t.Fatalf("error getting digest of manifest: %v", err)
	}

	// use this as the "bad" digest
	zeroDigest, err := digest.FromBytes([]byte{})
	if err != nil {
		t.Fatalf("error making zero digest: %v", err)
	}

	// Remote and local match, everything should look good
	local, _, _, err := store.loadManifest(signedBytes, dgst.String(), dgst)
	if err != nil {
		t.Fatalf("unexpected error verifying local and remote digest: %v", err)
	}

	if local != dgst {
		t.Fatalf("local digest not correctly calculated: %v", err)
	}

	// remote and no local, since pulling by tag
	local, _, _, err = store.loadManifest(signedBytes, "tag", dgst)
	if err != nil {
		t.Fatalf("unexpected error verifying tag pull and remote digest: %v", err)
	}

	if local != dgst {
		t.Fatalf("local digest not correctly calculated: %v", err)
	}

	// remote and differing local, this is the most important to fail
	local, _, _, err = store.loadManifest(signedBytes, zeroDigest.String(), dgst)
	if err == nil {
		t.Fatalf("error expected when verifying with differing local digest")
	}

	// no remote, no local (by tag)
	local, _, _, err = store.loadManifest(signedBytes, "tag", "")
	if err != nil {
		t.Fatalf("unexpected error verifying manifest without remote digest: %v", err)
	}

	if local != dgst {
		t.Fatalf("local digest not correctly calculated: %v", err)
	}

	// no remote, with local
	local, _, _, err = store.loadManifest(signedBytes, dgst.String(), "")
	if err != nil {
		t.Fatalf("unexpected error verifying manifest without remote digest: %v", err)
	}

	if local != dgst {
		t.Fatalf("local digest not correctly calculated: %v", err)
	}

	// bad remote, we fail the check.
	local, _, _, err = store.loadManifest(signedBytes, dgst.String(), zeroDigest)
	if err == nil {
		t.Fatalf("error expected when verifying with differing remote digest")
	}
}
