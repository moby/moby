package digest

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"testing"

	"github.com/docker/distribution/testutil"
)

func TestDigestVerifier(t *testing.T) {
	p := make([]byte, 1<<20)
	rand.Read(p)
	digest, err := FromBytes(p)
	if err != nil {
		t.Fatalf("unexpected error digesting bytes: %#v", err)
	}

	verifier, err := NewDigestVerifier(digest)
	if err != nil {
		t.Fatalf("unexpected error getting digest verifier: %s", err)
	}

	io.Copy(verifier, bytes.NewReader(p))

	if !verifier.Verified() {
		t.Fatalf("bytes not verified")
	}

	tf, tarSum, err := testutil.CreateRandomTarFile()
	if err != nil {
		t.Fatalf("error creating tarfile: %v", err)
	}

	digest, err = FromTarArchive(tf)
	if err != nil {
		t.Fatalf("error digesting tarsum: %v", err)
	}

	if digest.String() != tarSum {
		t.Fatalf("unexpected digest: %q != %q", digest.String(), tarSum)
	}

	expectedSize, _ := tf.Seek(0, os.SEEK_END) // Get tar file size
	tf.Seek(0, os.SEEK_SET)                    // seek back

	// This is the most relevant example for the registry application. It's
	// effectively a read through pipeline, where the final sink is the digest
	// verifier.
	verifier, err = NewDigestVerifier(digest)
	if err != nil {
		t.Fatalf("unexpected error getting digest verifier: %s", err)
	}

	lengthVerifier := NewLengthVerifier(expectedSize)
	rd := io.TeeReader(tf, lengthVerifier)
	io.Copy(verifier, rd)

	if !lengthVerifier.Verified() {
		t.Fatalf("verifier detected incorrect length")
	}

	if !verifier.Verified() {
		t.Fatalf("bytes not verified")
	}
}

// TODO(stevvooe): Add benchmarks to measure bytes/second throughput for
// DigestVerifier. We should be tarsum/gzip limited for common cases but we
// want to verify this.
//
// The relevant benchmarks for comparison can be run with the following
// commands:
//
// 	go test -bench . crypto/sha1
// 	go test -bench . github.com/docker/docker/pkg/tarsum
//
