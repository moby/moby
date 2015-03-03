package blobstore

import (
	"testing"
)

func TestRefNotExists(t *testing.T) {
	withTestStore(t, testRefNotExists)
}

func testRefNotExists(t *testing.T, testStore *localStore) {
	_, err := testStore.Ref(randomDigest(t), "shouldNotWork")
	if err == nil || !err.(Error).IsBlobNotExists() {
		t.Fatalf("expected %q error, got: %s", errDescriptions[errCodeBlobNotExists], err)
	}
}

func TestRef(t *testing.T) {
	withTestStore(t, testRef)
}

func testRef(t *testing.T, testStore *localStore) {
	refs := []string{"ref0", "ref1", "ref2", "ref3", "ref4"}

	d1 := writeRandomBlob(t, testStore, 20480, refs[0])

	ensureEqualReferences(t, d1.References(), refs[:1])

	for i := 0; i < len(refs); i++ {
		d2, err := testStore.Ref(d1.Digest(), refs[i])
		if err != nil {
			t.Fatalf("unable to add reference %q: %s", refs[i], err)
		}

		ensureEqualDescriptors(t, d1, d2, false)
		ensureEqualReferences(t, d2.References(), refs[:i+1])
	}
}

func TestDeref(t *testing.T) {
	withTestStore(t, testDeref)
}

func testDeref(t *testing.T, testStore *localStore) {
	refs := []string{"ref0", "ref1", "ref2", "ref3", "ref4"}
	d1 := writeRandomBlob(t, testStore, 20480, refs[0])

	for i := 1; i < len(refs); i++ {
		_, err := testStore.Ref(d1.Digest(), refs[i])
		if err != nil {
			t.Fatalf("unable to add reference %q: %s", refs[i], err)
		}
	}

	for i := len(refs) - 1; i >= 0; i-- {
		if err := testStore.Deref(d1.Digest(), refs[i]); err != nil {
			t.Fatalf("unable to deref %q: %s", refs[i], err)
		}

		if i > 0 {
			d2, err := testStore.Get(d1.Digest())
			if err != nil {
				t.Fatalf("unable to get blob %q: %s", d1.Digest(), err)
			}

			ensureEqualDescriptors(t, d1, d2, false)
			ensureEqualReferences(t, d2.References(), refs[:i])
		}
	}

	// The blob's references have all been removed
	// and the blob should no longer exist.
	_, err := testStore.Get(d1.Digest())
	if err == nil || !err.(Error).IsBlobNotExists() {
		t.Fatalf("expected %q error, got: %s", errDescriptions[errCodeBlobNotExists], err)
	}
}
