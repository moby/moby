package utils

import (
	"crypto/hmac"
	"encoding/hex"
	"errors"
	"fmt"
	gopath "path"
	"path/filepath"

	"github.com/endophage/gotuf/data"
)

var ErrWrongLength = errors.New("wrong length")

type ErrWrongHash struct {
	Type     string
	Expected []byte
	Actual   []byte
}

func (e ErrWrongHash) Error() string {
	return fmt.Sprintf("wrong %s hash, expected %#x got %#x", e.Type, e.Expected, e.Actual)
}

type ErrNoCommonHash struct {
	Expected data.Hashes
	Actual   data.Hashes
}

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

type ErrUnknownHashAlgorithm struct {
	Name string
}

func (e ErrUnknownHashAlgorithm) Error() string {
	return fmt.Sprintf("unknown hash algorithm: %s", e.Name)
}

type PassphraseFunc func(role string, confirm bool) ([]byte, error)

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

func NormalizeTarget(path string) string {
	return gopath.Join("/", path)
}

func HashedPaths(path string, hashes data.Hashes) []string {
	paths := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		hashedPath := filepath.Join(filepath.Dir(path), hex.EncodeToString(hash)+"."+filepath.Base(path))
		paths = append(paths, hashedPath)
	}
	return paths
}
