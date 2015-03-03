package blobstore

import (
	"testing"
)

func TestList(t *testing.T) {
	withTestStore(t, testList)
}

func testList(t *testing.T, testStore *localStore) {
	// Make 10 random content blobs.
	expectedDigests := make([]string, 10)
	for i := range expectedDigests {
		d := writeRandomBlob(t, testStore, 10240, "someRef")
		expectedDigests[i] = d.Digest()
	}

	blobDigests, err := testStore.List()
	if err != nil {
		t.Fatalf("could not list blob digests: %s", err)
	}

	ensureEqualReferences(t, expectedDigests, blobDigests)
}
