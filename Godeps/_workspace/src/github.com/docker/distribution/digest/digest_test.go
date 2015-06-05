package digest

import (
	"bytes"
	"io"
	"testing"
)

func TestParseDigest(t *testing.T) {
	for _, testcase := range []struct {
		input     string
		err       error
		algorithm Algorithm
		hex       string
	}{
		{
			input:     "tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
			algorithm: "tarsum+sha256",
			hex:       "e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
		},
		{
			input:     "tarsum.dev+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
			algorithm: "tarsum.dev+sha256",
			hex:       "e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
		},
		{
			input:     "tarsum.v1+sha256:220a60ecd4a3c32c282622a625a54db9ba0ff55b5ba9c29c7064a2bc358b6a3e",
			algorithm: "tarsum.v1+sha256",
			hex:       "220a60ecd4a3c32c282622a625a54db9ba0ff55b5ba9c29c7064a2bc358b6a3e",
		},
		{
			input:     "sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
			algorithm: "sha256",
			hex:       "e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
		},
		{
			input:     "sha384:d3fc7881460b7e22e3d172954463dddd7866d17597e7248453c48b3e9d26d9596bf9c4a9cf8072c9d5bad76e19af801d",
			algorithm: "sha384",
			hex:       "d3fc7881460b7e22e3d172954463dddd7866d17597e7248453c48b3e9d26d9596bf9c4a9cf8072c9d5bad76e19af801d",
		},
		{
			// empty hex
			input: "sha256:",
			err:   ErrDigestInvalidFormat,
		},
		{
			// just hex
			input: "d41d8cd98f00b204e9800998ecf8427e",
			err:   ErrDigestInvalidFormat,
		},
		{
			// not hex
			input: "sha256:d41d8cd98f00b204e9800m98ecf8427e",
			err:   ErrDigestInvalidFormat,
		},
		{
			input: "foo:d41d8cd98f00b204e9800998ecf8427e",
			err:   ErrDigestUnsupported,
		},
	} {
		digest, err := ParseDigest(testcase.input)
		if err != testcase.err {
			t.Fatalf("error differed from expected while parsing %q: %v != %v", testcase.input, err, testcase.err)
		}

		if testcase.err != nil {
			continue
		}

		if digest.Algorithm() != testcase.algorithm {
			t.Fatalf("incorrect algorithm for parsed digest: %q != %q", digest.Algorithm(), testcase.algorithm)
		}

		if digest.Hex() != testcase.hex {
			t.Fatalf("incorrect hex for parsed digest: %q != %q", digest.Hex(), testcase.hex)
		}

		// Parse string return value and check equality
		newParsed, err := ParseDigest(digest.String())

		if err != nil {
			t.Fatalf("unexpected error parsing input %q: %v", testcase.input, err)
		}

		if newParsed != digest {
			t.Fatalf("expected equal: %q != %q", newParsed, digest)
		}
	}
}

// A few test cases used to fix behavior we expect in storage backend.

func TestFromTarArchiveZeroLength(t *testing.T) {
	checkTarsumDigest(t, "zero-length archive", bytes.NewReader([]byte{}), DigestTarSumV1EmptyTar)
}

func TestFromTarArchiveEmptyTar(t *testing.T) {
	// String of 1024 zeros is a valid, empty tar file.
	checkTarsumDigest(t, "1024 zero bytes", bytes.NewReader(bytes.Repeat([]byte("\x00"), 1024)), DigestTarSumV1EmptyTar)
}

func checkTarsumDigest(t *testing.T, msg string, rd io.Reader, expected Digest) {
	dgst, err := FromTarArchive(rd)
	if err != nil {
		t.Fatalf("unexpected error digesting %s: %v", msg, err)
	}

	if dgst != expected {
		t.Fatalf("unexpected digest for %s: %q != %q", msg, dgst, expected)
	}
}
