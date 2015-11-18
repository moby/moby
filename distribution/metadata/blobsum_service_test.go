package metadata

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/layer"
)

func TestBlobSumService(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "blobsum-storage-service-test")
	if err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	metadataStore, err := NewFSMetadataStore(tmpDir)
	if err != nil {
		t.Fatalf("could not create metadata store: %v", err)
	}
	blobSumService := NewBlobSumService(metadataStore)

	testVectors := []struct {
		diffID   layer.DiffID
		blobsums []digest.Digest
	}{
		{
			diffID: layer.DiffID("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"),
			blobsums: []digest.Digest{
				digest.Digest("sha256:f0cd5ca10b07f35512fc2f1cbf9a6cefbdb5cba70ac6b0c9e5988f4497f71937"),
			},
		},
		{
			diffID: layer.DiffID("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa"),
			blobsums: []digest.Digest{
				digest.Digest("sha256:f0cd5ca10b07f35512fc2f1cbf9a6cefbdb5cba70ac6b0c9e5988f4497f71937"),
				digest.Digest("sha256:9e3447ca24cb96d86ebd5960cb34d1299b07e0a0e03801d90b9969a2c187dd6e"),
			},
		},
		{
			diffID: layer.DiffID("sha256:03f4658f8b782e12230c1783426bd3bacce651ce582a4ffb6fbbfa2079428ecb"),
			blobsums: []digest.Digest{
				digest.Digest("sha256:f0cd5ca10b07f35512fc2f1cbf9a6cefbdb5cba70ac6b0c9e5988f4497f71937"),
				digest.Digest("sha256:9e3447ca24cb96d86ebd5960cb34d1299b07e0a0e03801d90b9969a2c187dd6e"),
				digest.Digest("sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf"),
				digest.Digest("sha256:8902a7ca89aabbb868835260912159026637634090dd8899eee969523252236e"),
				digest.Digest("sha256:c84364306344ccc48532c52ff5209236273525231dddaaab53262322352883aa"),
				digest.Digest("sha256:aa7583bbc87532a8352bbb72520a821b3623523523a8352523a52352aaa888fe"),
			},
		},
	}

	// Set some associations
	for _, vec := range testVectors {
		for _, blobsum := range vec.blobsums {
			err := blobSumService.Add(vec.diffID, blobsum)
			if err != nil {
				t.Fatalf("error calling Set: %v", err)
			}
		}
	}

	// Check the correct values are read back
	for _, vec := range testVectors {
		blobsums, err := blobSumService.GetBlobSums(vec.diffID)
		if err != nil {
			t.Fatalf("error calling Get: %v", err)
		}
		expectedBlobsums := len(vec.blobsums)
		if expectedBlobsums > 5 {
			expectedBlobsums = 5
		}
		if !reflect.DeepEqual(blobsums, vec.blobsums[len(vec.blobsums)-expectedBlobsums:len(vec.blobsums)]) {
			t.Fatal("Get returned incorrect layer ID")
		}
	}

	// Test GetBlobSums on a nonexistent entry
	_, err = blobSumService.GetBlobSums(layer.DiffID("sha256:82379823067823853223359023576437723560923756b03560378f4497753917"))
	if err == nil {
		t.Fatal("expected error looking up nonexistent entry")
	}

	// Test GetDiffID on a nonexistent entry
	_, err = blobSumService.GetDiffID(digest.Digest("sha256:82379823067823853223359023576437723560923756b03560378f4497753917"))
	if err == nil {
		t.Fatal("expected error looking up nonexistent entry")
	}

	// Overwrite one of the entries and read it back
	err = blobSumService.Add(testVectors[1].diffID, testVectors[0].blobsums[0])
	if err != nil {
		t.Fatalf("error calling Add: %v", err)
	}
	diffID, err := blobSumService.GetDiffID(testVectors[0].blobsums[0])
	if err != nil {
		t.Fatalf("error calling GetDiffID: %v", err)
	}
	if diffID != testVectors[1].diffID {
		t.Fatal("GetDiffID returned incorrect diffID")
	}
}
