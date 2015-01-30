package graph

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

const (
	testManifestImageName    = "testapp"
	testManifestImageID      = "d821b739e8834ec89ac4469266c3d11515da88fdcbcbdddcbcddb636f54fdde9"
	testManifestImageIDShort = "d821b739e883"
	testManifestTag          = "manifesttest"
)

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
