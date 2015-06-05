package digest

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"strings"
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

// TestVerifierUnsupportedDigest ensures that unsupported digest validation is
// flowing through verifier creation.
func TestVerifierUnsupportedDigest(t *testing.T) {
	unsupported := Digest("bean:0123456789abcdef")

	_, err := NewDigestVerifier(unsupported)
	if err == nil {
		t.Fatalf("expected error when creating verifier")
	}

	if err != ErrDigestUnsupported {
		t.Fatalf("incorrect error for unsupported digest: %v", err)
	}
}

// TestJunkNoDeadlock ensures that junk input into a digest verifier properly
// returns errors from the tarsum library. Specifically, we pass in a file
// with a "bad header" and should see the error from the io.Copy to verifier.
// This has been seen with gzipped tarfiles, mishandled by the tarsum package,
// but also on junk input, such as html.
func TestJunkNoDeadlock(t *testing.T) {
	expected := Digest("tarsum.dev+sha256:62e15750aae345f6303469a94892e66365cc5e3abdf8d7cb8b329f8fb912e473")
	junk := bytes.Repeat([]byte{'a'}, 1024)

	verifier, err := NewDigestVerifier(expected)
	if err != nil {
		t.Fatalf("unexpected error creating verifier: %v", err)
	}

	rd := bytes.NewReader(junk)
	if _, err := io.Copy(verifier, rd); err == nil {
		t.Fatalf("unexpected error verifying input data: %v", err)
	}
}

// TestBadTarNoDeadlock runs a tar with a "bad" tar header through digest
// verifier, ensuring that the verifier returns an error properly.
func TestBadTarNoDeadlock(t *testing.T) {
	// TODO(stevvooe): This test is exposing a bug in tarsum where if we pass
	// a gzipped tar file into tarsum, the library returns an error. This
	// should actually work. When the tarsum package is fixed, this test will
	// fail and we can remove this test or invert it.

	// This tarfile was causing deadlocks in verifiers due mishandled copy error.
	// This is a gzipped tar, which we typically don't see but should handle.
	//
	// From https://registry-1.docker.io/v2/library/ubuntu/blobs/tarsum.dev+sha256:62e15750aae345f6303469a94892e66365cc5e3abdf8d7cb8b329f8fb912e473
	const badTar = `
H4sIAAAJbogA/0otSdZnoDEwMDAxMDc1BdJggE6D2YZGJobGBmbGRsZAdYYGBkZGDAqmtHYYCJQW
lyQWAZ1CqTnonhsiAAAAAP//AsV/YkEJTdMAGfFvZmA2Gv/0AAAAAAD//4LFf3F+aVFyarFeTmZx
CbXtAOVnMxMTXPFvbGpmjhb/xobmwPinSyCO8PgHAAAA///EVU9v2z4MvedTEMihl9a5/26/YTkU
yNKiTTDsKMt0rE0WDYmK628/ym7+bFmH2DksQACbIB/5+J7kObwiQsXc/LdYVGibLObRccw01Qv5
19EZ7hbbZudVgWtiDFCSh4paYII4xOVxNgeHLXrYow+GXAAqgSuEQhzlTR5ZgtlsVmB+aKe8rswe
zzsOjwtoPGoTEGplHHhMCJqxSNUPwesbEGbzOXxR34VCHndQmjfhUKhEq/FURI0FqJKFR5q9NE5Z
qbaoBGoglAB+5TSK0sOh3c3UPkRKE25dEg8dDzzIWmqN2wG3BNY4qRL1VFFAoJJb5SXHU90n34nk
SUS8S0AeGwqGyXdZel1nn7KLGhPO0kDeluvN48ty9Q2269ft8/PTy2b5GfKuh9/2LBIWo6oz+N8G
uodmWLETg0mW4lMP4XYYCL4+rlawftpIO40SA+W6Yci9wRZE1MNOjmyGdhBQRy9OHpqOdOGh/wT7
nZdOkHZ650uIK+WrVZdkgErJfnNEJysLnI5FSAj4xuiCQNpOIoNWmhyLByVHxEpLf3dkr+k9KMsV
xV0FhiVB21hgD3V5XwSqRdOmsUYr7oNtZXTVzyTHc2/kqokBy2ihRMVRTN+78goP5Ur/aMhz+KOJ
3h2UsK43kdwDo0Q9jfD7ie2RRur7MdpIrx1Z3X4j/Q1qCswN9r/EGCvXiUy0fI4xeSknnH/92T/+
fgIAAP//GkWjYBSMXAAIAAD//2zZtzAAEgAA`
	expected := Digest("tarsum.dev+sha256:62e15750aae345f6303469a94892e66365cc5e3abdf8d7cb8b329f8fb912e473")

	verifier, err := NewDigestVerifier(expected)
	if err != nil {
		t.Fatalf("unexpected error creating verifier: %v", err)
	}

	rd := base64.NewDecoder(base64.StdEncoding, strings.NewReader(badTar))

	if _, err := io.Copy(verifier, rd); err == nil {
		t.Fatalf("unexpected error verifying input data: %v", err)
	}

	if verifier.Verified() {
		// For now, we expect an error, since tarsum library cannot handle
		// compressed tars (!!!).
		t.Fatalf("no error received after invalid tar")
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
