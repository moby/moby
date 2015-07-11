package digest

import (
	"hash"
	"io"
	"io/ioutil"

	"github.com/docker/docker/pkg/tarsum"
)

// Verifier presents a general verification interface to be used with message
// digests and other byte stream verifications. Users instantiate a Verifier
// from one of the various methods, write the data under test to it then check
// the result with the Verified method.
type Verifier interface {
	io.Writer

	// Verified will return true if the content written to Verifier matches
	// the digest.
	Verified() bool
}

// NewDigestVerifier returns a verifier that compares the written bytes
// against a passed in digest.
func NewDigestVerifier(d Digest) (Verifier, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}

	alg := d.Algorithm()
	switch alg {
	case "sha256", "sha384", "sha512":
		return hashVerifier{
			hash:   alg.Hash(),
			digest: d,
		}, nil
	default:
		// Assume we have a tarsum.
		version, err := tarsum.GetVersionFromTarsum(string(d))
		if err != nil {
			return nil, err
		}

		pr, pw := io.Pipe()

		// TODO(stevvooe): We may actually want to ban the earlier versions of
		// tarsum. That decision may not be the place of the verifier.

		ts, err := tarsum.NewTarSum(pr, true, version)
		if err != nil {
			return nil, err
		}

		// TODO(sday): Ick! A goroutine per digest verification? We'll have to
		// get the tarsum library to export an io.Writer variant.
		go func() {
			if _, err := io.Copy(ioutil.Discard, ts); err != nil {
				pr.CloseWithError(err)
			} else {
				pr.Close()
			}
		}()

		return &tarsumVerifier{
			digest: d,
			ts:     ts,
			pr:     pr,
			pw:     pw,
		}, nil
	}
}

// NewLengthVerifier returns a verifier that returns true when the number of
// read bytes equals the expected parameter.
func NewLengthVerifier(expected int64) Verifier {
	return &lengthVerifier{
		expected: expected,
	}
}

type lengthVerifier struct {
	expected int64 // expected bytes read
	len      int64 // bytes read
}

func (lv *lengthVerifier) Write(p []byte) (n int, err error) {
	n = len(p)
	lv.len += int64(n)
	return n, err
}

func (lv *lengthVerifier) Verified() bool {
	return lv.expected == lv.len
}

type hashVerifier struct {
	digest Digest
	hash   hash.Hash
}

func (hv hashVerifier) Write(p []byte) (n int, err error) {
	return hv.hash.Write(p)
}

func (hv hashVerifier) Verified() bool {
	return hv.digest == NewDigest(hv.digest.Algorithm(), hv.hash)
}

type tarsumVerifier struct {
	digest Digest
	ts     tarsum.TarSum
	pr     *io.PipeReader
	pw     *io.PipeWriter
}

func (tv *tarsumVerifier) Write(p []byte) (n int, err error) {
	return tv.pw.Write(p)
}

func (tv *tarsumVerifier) Verified() bool {
	return tv.digest == Digest(tv.ts.Sum(nil))
}
