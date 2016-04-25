package utils

import (
	"crypto/hmac"
	"encoding/hex"
	"errors"
	"fmt"
	gopath "path"
	"path/filepath"

	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/tuf/data"
)

// ErrWrongLength indicates the length was different to that expected
var ErrWrongLength = errors.New("wrong length")

// ErrWrongHash indicates the hash was different to that expected
type ErrWrongHash struct {
	Type     string
	Expected []byte
	Actual   []byte
}

// Error implements error interface
func (e ErrWrongHash) Error() string {
	return fmt.Sprintf("wrong %s hash, expected %#x got %#x", e.Type, e.Expected, e.Actual)
}

// ErrNoCommonHash indicates the metadata did not provide any hashes this
// client recognizes
type ErrNoCommonHash struct {
	Expected data.Hashes
	Actual   data.Hashes
}

// Error implements error interface
func (e ErrNoCommonHash) Error() string {
	types := func(a data.Hashes) []string {
		t := make([]string, 0, len(a))
		for typ := range a {
			t = append(t, typ)
		}
		return t
	}
	return fmt.Sprintf("no common hash function, expected one of %s, got %s", types(e.Expected), types(e.Actual))
}

// ErrUnknownHashAlgorithm - client was ashed to use a hash algorithm
// it is not familiar with
type ErrUnknownHashAlgorithm struct {
	Name string
}

// Error implements error interface
func (e ErrUnknownHashAlgorithm) Error() string {
	return fmt.Sprintf("unknown hash algorithm: %s", e.Name)
}

// PassphraseFunc type for func that request a passphrase
type PassphraseFunc func(role string, confirm bool) ([]byte, error)

// FileMetaEqual checks whether 2 FileMeta objects are consistent with eachother
func FileMetaEqual(actual data.FileMeta, expected data.FileMeta) error {
	if actual.Length != expected.Length {
		return ErrWrongLength
	}
	hashChecked := false
	for typ, hash := range expected.Hashes {
		if h, ok := actual.Hashes[typ]; ok {
			hashChecked = true
			if !hmac.Equal(h, hash) {
				return ErrWrongHash{typ, hash, h}
			}
		}
	}
	if !hashChecked {
		return ErrNoCommonHash{expected.Hashes, actual.Hashes}
	}
	return nil
}

// NormalizeTarget adds a slash, if required, to the front of a target path
func NormalizeTarget(path string) string {
	return gopath.Join("/", path)
}

// HashedPaths prefixes the filename with the known hashes for the file,
// returning a list of possible consistent paths.
func HashedPaths(path string, hashes data.Hashes) []string {
	paths := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		hashedPath := filepath.Join(filepath.Dir(path), hex.EncodeToString(hash)+"."+filepath.Base(path))
		paths = append(paths, hashedPath)
	}
	return paths
}

// CanonicalKeyID returns the ID of the public bytes version of a TUF key.
// On regular RSA/ECDSA TUF keys, this is just the key ID.  On X509 RSA/ECDSA
// TUF keys, this is the key ID of the public key part of the key in the leaf cert
func CanonicalKeyID(k data.PublicKey) (string, error) {
	switch k.Algorithm() {
	case data.ECDSAx509Key, data.RSAx509Key:
		return trustmanager.X509PublicKeyID(k)
	default:
		return k.ID(), nil
	}
}
