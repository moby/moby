package fernet

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

var (
	errKeyLen = errors.New("fernet: key decodes to wrong size")
	errNoKeys = errors.New("fernet: no keys provided")
)

// Key represents a key.
type Key [32]byte

func (k *Key) cryptBytes() []byte {
	return k[len(k)/2:]
}

func (k *Key) signBytes() []byte {
	return k[:len(k)/2]
}

// Generate initializes k with pseudorandom data from package crypto/rand.
func (k *Key) Generate() error {
	_, err := io.ReadFull(rand.Reader, k[:])
	return err
}

// Encode returns the URL-safe base64 encoding of k.
func (k *Key) Encode() string {
	return encoding.EncodeToString(k[:])
}

// DecodeKey decodes a key from s and returns it. The key can be in
// hexadecimal, standard base64, or URL-safe base64.
func DecodeKey(s string) (*Key, error) {
	var b []byte
	var err error
	if s == "" {
		return nil, errors.New("empty key")
	}
	if len(s) == hex.EncodedLen(len(Key{})) {
		b, err = hex.DecodeString(s)
	} else {
		b, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			b, err = base64.URLEncoding.DecodeString(s)
		}
	}
	if err != nil {
		return nil, err
	}
	if len(b) != len(Key{}) {
		return nil, errKeyLen
	}
	k := new(Key)
	copy(k[:], b)
	return k, nil
}

// DecodeKeys decodes each element of a using DecodeKey and returns the
// resulting keys. Requires at least one key.
func DecodeKeys(a ...string) ([]*Key, error) {
	if len(a) == 0 {
		return nil, errNoKeys
	}
	var err error
	ks := make([]*Key, len(a))
	for i, s := range a {
		ks[i], err = DecodeKey(s)
		if err != nil {
			return nil, err
		}
	}
	return ks, nil
}

// MustDecodeKeys is like DecodeKeys, but panics if an error occurs.
// It simplifies safe initialization of global variables holding
// keys.
func MustDecodeKeys(a ...string) []*Key {
	k, err := DecodeKeys(a...)
	if err != nil {
		panic(err)
	}
	return k
}
