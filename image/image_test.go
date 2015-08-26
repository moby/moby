package image

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/docker/distribution/digest"
)

var fixtures = []string{
	"fixtures/pre1.9",
	"fixtures/post1.9",
}

func loadFixtureFile(t *testing.T, path string) []byte {
	fileData, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("error opening %s: %v", path, err)
	}

	return bytes.TrimSpace(fileData)
}

// TestMakeImageConfig makes sure that MakeImageConfig returns the expected
// canonical JSON for a reference Image.
func TestMakeImageConfig(t *testing.T) {
	for _, fixture := range fixtures {
		v1Compatibility := loadFixtureFile(t, fixture+"/v1compatibility")
		expectedConfig := loadFixtureFile(t, fixture+"/expected_config")
		layerID := digest.Digest(loadFixtureFile(t, fixture+"/layer_id"))
		parentID := digest.Digest(loadFixtureFile(t, fixture+"/parent_id"))

		json, err := MakeImageConfig(v1Compatibility, layerID, parentID)
		if err != nil {
			t.Fatalf("MakeImageConfig on %s returned error: %v", fixture, err)
		}
		if !bytes.Equal(json, expectedConfig) {
			t.Fatalf("did not get expected JSON for %s\nexpected: %s\ngot: %s", fixture, expectedConfig, json)
		}
	}
}

// TestGetStrongID makes sure that GetConfigJSON returns the expected
// hash for a reference Image.
func TestGetStrongID(t *testing.T) {
	for _, fixture := range fixtures {
		expectedConfig := loadFixtureFile(t, fixture+"/expected_config")
		expectedComputedID := digest.Digest(loadFixtureFile(t, fixture+"/expected_computed_id"))

		if id, err := StrongID(expectedConfig); err != nil || id != expectedComputedID {
			t.Fatalf("did not get expected ID for %s\nexpected: %s\ngot: %s\nerror: %v", fixture, expectedComputedID, id, err)
		}
	}
}
