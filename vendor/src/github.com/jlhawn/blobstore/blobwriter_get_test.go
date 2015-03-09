package blobstore

import (
	"testing"
)

func TestGetNotExists(t *testing.T) {
	withTestStore(t, testGetNotExists)
}

func testGetNotExists(t *testing.T, testStore *localStore) {
	_, err := testStore.Get(randomDigest(t))
	if err == nil || !err.(Error).IsBlobNotExists() {
		t.Fatalf("expected %q error, got: %s", errDescriptions[errCodeBlobNotExists], err)
	}
}

func TestWriteGetRoundtrip(t *testing.T) {
	withTestStore(t, testWriteGetRoundtrip)
}

func testWriteGetRoundtrip(t *testing.T, testStore *localStore) {
	refID := "testWriteGetRoundtripRef"

	d := writeRandomBlob(t, testStore, 20480, refID)

	b, err := testStore.Get(d.Digest())
	if err != nil {
		t.Fatalf("unable to get blob: %s", err)
	}

	if b.Size() != d.Size() {
		t.Fatalf("expected blob size to be %d, got %d", d.Size(), b.Size())
	}

	if b.MediaType() != randomDataMediaType {
		t.Fatalf("expected blob media type to be %d, got %d", randomDataMediaType, b.MediaType())
	}

	if b.Digest() != d.Digest() {
		t.Fatalf("put/get digest conflict: put %q, god %q", d.Digest(), b.Digest())
	}
}
