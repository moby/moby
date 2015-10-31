package digest

import (
	"bytes"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/tarsum"
)

const (
	// DigestTarSumV1EmptyTar is the digest for the empty tar file.
	DigestTarSumV1EmptyTar = "tarsum.v1+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// DigestSha256EmptyTar is the canonical sha256 digest of empty data
	DigestSha256EmptyTar = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// Digest allows simple protection of hex formatted digest strings, prefixed
// by their algorithm. Strings of type Digest have some guarantee of being in
// the correct format and it provides quick access to the components of a
// digest string.
//
// The following is an example of the contents of Digest types:
//
// 	sha256:7173b809ca12ec5dee4506cd86be934c4596dd234ee82c0662eac04a8c2c71dc
//
// More important for this code base, this type is compatible with tarsum
// digests. For example, the following would be a valid Digest:
//
// 	tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b
//
// This allows to abstract the digest behind this type and work only in those
// terms.
type Digest string

// NewDigest returns a Digest from alg and a hash.Hash object.
func NewDigest(alg Algorithm, h hash.Hash) Digest {
	return Digest(fmt.Sprintf("%s:%x", alg, h.Sum(nil)))
}

// NewDigestFromHex returns a Digest from alg and a the hex encoded digest.
func NewDigestFromHex(alg, hex string) Digest {
	return Digest(fmt.Sprintf("%s:%s", alg, hex))
}

// DigestRegexp matches valid digest types.
var DigestRegexp = regexp.MustCompile(`[a-zA-Z0-9-_+.]+:[a-fA-F0-9]+`)

// DigestRegexpAnchored matches valid digest types, anchored to the start and end of the match.
var DigestRegexpAnchored = regexp.MustCompile(`^` + DigestRegexp.String() + `$`)

var (
	// ErrDigestInvalidFormat returned when digest format invalid.
	ErrDigestInvalidFormat = fmt.Errorf("invalid checksum digest format")

	// ErrDigestUnsupported returned when the digest algorithm is unsupported.
	ErrDigestUnsupported = fmt.Errorf("unsupported digest algorithm")
)

// ParseDigest parses s and returns the validated digest object. An error will
// be returned if the format is invalid.
func ParseDigest(s string) (Digest, error) {
	d := Digest(s)

	return d, d.Validate()
}

// FromReader returns the most valid digest for the underlying content using
// the canonical digest algorithm.
func FromReader(rd io.Reader) (Digest, error) {
	return Canonical.FromReader(rd)
}

// FromTarArchive produces a tarsum digest from reader rd.
func FromTarArchive(rd io.Reader) (Digest, error) {
	ts, err := tarsum.NewTarSum(rd, true, tarsum.Version1)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(ioutil.Discard, ts); err != nil {
		return "", err
	}

	d, err := ParseDigest(ts.Sum(nil))
	if err != nil {
		return "", err
	}

	return d, nil
}

// FromBytes digests the input and returns a Digest.
func FromBytes(p []byte) (Digest, error) {
	return FromReader(bytes.NewReader(p))
}

// Validate checks that the contents of d is a valid digest, returning an
// error if not.
func (d Digest) Validate() error {
	s := string(d)
	// Common case will be tarsum
	_, err := ParseTarSum(s)
	if err == nil {
		return nil
	}

	// Continue on for general parser

	if !DigestRegexpAnchored.MatchString(s) {
		return ErrDigestInvalidFormat
	}

	i := strings.Index(s, ":")
	if i < 0 {
		return ErrDigestInvalidFormat
	}

	// case: "sha256:" with no hex.
	if i+1 == len(s) {
		return ErrDigestInvalidFormat
	}

	switch Algorithm(s[:i]) {
	case SHA256, SHA384, SHA512:
		break
	default:
		return ErrDigestUnsupported
	}

	return nil
}

// Algorithm returns the algorithm portion of the digest. This will panic if
// the underlying digest is not in a valid format.
func (d Digest) Algorithm() Algorithm {
	return Algorithm(d[:d.sepIndex()])
}

// Hex returns the hex digest portion of the digest. This will panic if the
// underlying digest is not in a valid format.
func (d Digest) Hex() string {
	return string(d[d.sepIndex()+1:])
}

func (d Digest) String() string {
	return string(d)
}

func (d Digest) sepIndex() int {
	i := strings.Index(string(d), ":")

	if i < 0 {
		panic("could not find ':' in digest: " + d)
	}

	return i
}
