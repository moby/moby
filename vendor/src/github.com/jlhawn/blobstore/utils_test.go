package blobstore

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

// withTestStore will create a new test store and call the given test function
// and remove the test store ONLY if the test passes.
func withTestStore(t *testing.T, testFunc func(*testing.T, *localStore)) {
	testStore := newTestStore(t)

	testFunc(t, testStore)

	if !t.Failed() {
		os.RemoveAll(testStore.root)
	}
}

func newTestStore(t *testing.T) *localStore {
	tempDirname, err := ioutil.TempDir(".", "temp-local-blob-store-")
	if err != nil {
		t.Fatal(err)
	}

	ls, err := newLocalStore(tempDirname)
	if err != nil {
		t.Fatal(err)
	}

	return ls
}

func writeRandomBlob(t *testing.T, testStore *localStore, blobSize uint64, refID string) Descriptor {
	bw, err := testStore.NewWriter(HashForLabel("sha256"))
	if err != nil {
		t.Fatalf("unable to make new blob writer: %s", err)
	}

	randData := limitedRandReader(blobSize)
	hasher := sha256.New()

	if _, err := io.Copy(bw, io.TeeReader(randData, hasher)); err != nil {
		t.Fatalf("unable to write random data blob: %s", err)
	}

	digest := fmt.Sprintf("sha256:%x", hasher.Sum(nil))

	d, err := bw.Commit(randomDataMediaType, refID)
	if err != nil {
		t.Fatalf("unable to commit put of random blob: %s", err)
	}

	info := blobInfo{
		Digest:    digest,
		MediaType: randomDataMediaType,
		Size:      blobSize,
	}

	ensureEqualDescriptors(t, d, newDescriptor(info), false)

	return d
}

func ensureEqualDescriptors(t *testing.T, d1, d2 Descriptor, checkRefs bool) {
	if d1.Digest() != d2.Digest() {
		t.Fatalf("digest mismatch: %q != %q", d1.Digest(), d2.Digest())
	}

	if d1.Size() != d2.Size() {
		t.Fatalf("blob size mismatch: %d != %d", d1.Size(), d2.Size())
	}

	if d1.MediaType() != d2.MediaType() {
		t.Fatalf("media type mismatch: %q != %q", d1.MediaType(), d2.MediaType())
	}

	if checkRefs {
		ensureEqualReferences(t, d1.References(), d2.References())
	}
}

func ensureEqualReferences(t *testing.T, refs1, refs2 []string) {
	if len(refs1) != len(refs2) {
		t.Fatalf("refs length mismatch: %d != %d", len(refs1), len(refs2))
	}

	refSet1 := make(map[string]struct{}, len(refs1))
	for _, ref := range refs1 {
		refSet1[ref] = struct{}{}
	}

	refSet2 := make(map[string]struct{}, len(refs2))
	for _, ref := range refs2 {
		refSet2[ref] = struct{}{}
	}

	if len(refSet1) != len(refSet2) {
		t.Fatalf("ref set size mismatch: %d != %d", len(refSet1), len(refSet2))
	}

	for ref := range refSet1 {
		if _, ok := refSet2[ref]; !ok {
			t.Fatalf("ref set does not contain %q, has: %s", ref, refs2)
		}
	}
}

func limitedRandReader(length uint64) io.Reader {
	return io.LimitReader(rand.Reader, int64(length))
}

var randomDataMediaType = "application/random.data+octet-stream"

func randomDigest(t *testing.T) string {
	return fmt.Sprintf("sha256:%s", randomHexString(t))
}

func randomHexString(t *testing.T) string {
	// Get 32 random bytes to make something that looks like a sha256 digest.
	var buf [32]byte

	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		t.Fatal(err)
	}

	return fmt.Sprintf("%x", buf[:])
}
