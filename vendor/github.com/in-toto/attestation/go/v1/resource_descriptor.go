/*
Wrapper APIs for in-toto attestation ResourceDescriptor protos.
*/

package v1

import (
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrIncorrectDigestLength = errors.New("digest has incorrect length")
	ErrInvalidDigestEncoding = errors.New("digest is not valid hex-encoded string")
	ErrRDRequiredField       = errors.New("at least one of name, URI, or digest are required")
)

type HashAlgorithm string

const (
	AlgorithmMD5        HashAlgorithm = "md5"
	AlgorithmSHA1       HashAlgorithm = "sha1"
	AlgorithmSHA224     HashAlgorithm = "sha224"
	AlgorithmSHA512_224 HashAlgorithm = "sha512_224"
	AlgorithmSHA256     HashAlgorithm = "sha256"
	AlgorithmSHA512_256 HashAlgorithm = "sha512_256"
	AlgorithmSHA384     HashAlgorithm = "sha384"
	AlgorithmSHA512     HashAlgorithm = "sha512"
	AlgorithmSHA3_224   HashAlgorithm = "sha3_224"
	AlgorithmSHA3_256   HashAlgorithm = "sha3_256"
	AlgorithmSHA3_384   HashAlgorithm = "sha3_384"
	AlgorithmSHA3_512   HashAlgorithm = "sha3_512"
	AlgorithmGitBlob    HashAlgorithm = "gitBlob"
	AlgorithmGitCommit  HashAlgorithm = "gitCommit"
	AlgorithmGitTag     HashAlgorithm = "gitTag"
	AlgorithmGitTree    HashAlgorithm = "gitTree"
	AlgorithmDirHash    HashAlgorithm = "dirHash"
)

// HashAlgorithms indexes the known algorithms in a dictionary
// by their string value
var HashAlgorithms = map[string]HashAlgorithm{
	"md5":        AlgorithmMD5,
	"sha1":       AlgorithmSHA1,
	"sha224":     AlgorithmSHA224,
	"sha512_224": AlgorithmSHA512_224,
	"sha256":     AlgorithmSHA256,
	"sha512_256": AlgorithmSHA512_256,
	"sha384":     AlgorithmSHA384,
	"sha512":     AlgorithmSHA512,
	"sha3_224":   AlgorithmSHA3_224,
	"sha3_256":   AlgorithmSHA3_256,
	"sha3_384":   AlgorithmSHA3_384,
	"sha3_512":   AlgorithmSHA3_512,
	"gitBlob":    AlgorithmGitBlob,
	"gitCommit":  AlgorithmGitCommit,
	"gitTag":     AlgorithmGitTag,
	"gitTree":    AlgorithmGitTree,
	"dirHash":    AlgorithmDirHash,
}

// HexLength returns the expected length of an algorithm's hash when hexencoded
func (algo HashAlgorithm) HexLength() int {
	switch algo {
	case AlgorithmMD5:
		return 16
	case AlgorithmSHA1, AlgorithmGitBlob, AlgorithmGitCommit, AlgorithmGitTag, AlgorithmGitTree:
		return 20
	case AlgorithmSHA224, AlgorithmSHA512_224, AlgorithmSHA3_224:
		return 28
	case AlgorithmSHA256, AlgorithmSHA512_256, AlgorithmSHA3_256, AlgorithmDirHash:
		return 32
	case AlgorithmSHA384, AlgorithmSHA3_384:
		return 48
	case AlgorithmSHA512, AlgorithmSHA3_512:
		return 64
	default:
		return 0
	}
}

// String returns the hash algorithm name as a string
func (algo HashAlgorithm) String() string {
	return string(algo)
}

// Indicates if a given fixed-size hash algorithm is supported by default and returns the algorithm's
// digest size in bytes, if supported. We assume gitCommit and dirHash are aliases for sha1 and sha256, respectively.
//
// SHA digest sizes from https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.202.pdf
// MD5 digest size from https://www.rfc-editor.org/rfc/rfc1321.html#section-1
func isSupportedFixedSizeAlgorithm(algString string) (bool, int) {
	algo := HashAlgorithm(algString)
	return algo.HexLength() > 0, algo.HexLength()
}

func (d *ResourceDescriptor) Validate() error {
	// at least one of name, URI or digest are required
	if d.GetName() == "" && d.GetUri() == "" && len(d.GetDigest()) == 0 {
		return ErrRDRequiredField
	}

	if len(d.GetDigest()) > 0 {
		for alg, digest := range d.GetDigest() {

			// Per https://github.com/in-toto/attestation/blob/main/spec/v1/digest_set.md
			// check encoding and length for supported algorithms;
			// use of custom, unsupported algorithms is allowed and does not not generate validation errors.
			supported, size := isSupportedFixedSizeAlgorithm(alg)
			if supported {
				// the in-toto spec expects a hex-encoded string in DigestSets for supported algorithms
				hashBytes, err := hex.DecodeString(digest)

				if err != nil {
					return fmt.Errorf("%w (%s: %s)", ErrInvalidDigestEncoding, alg, digest)
				}

				// check the length of the digest
				if len(hashBytes) != size {
					return fmt.Errorf("%w: got %d bytes, want %d bytes (%s: %s)", ErrIncorrectDigestLength, len(hashBytes), size, alg, digest)
				}
			}
		}
	}

	return nil
}
