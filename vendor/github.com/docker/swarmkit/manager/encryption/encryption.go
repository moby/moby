package encryption

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
)

// This package defines the interfaces and encryption package

const humanReadablePrefix = "SWMKEY-1-"

// ErrCannotDecrypt is the type of error returned when some data cannot be decryptd as plaintext
type ErrCannotDecrypt struct {
	msg string
}

func (e ErrCannotDecrypt) Error() string {
	return e.msg
}

// A Decrypter can decrypt an encrypted record
type Decrypter interface {
	Decrypt(api.MaybeEncryptedRecord) ([]byte, error)
}

// A Encrypter can encrypt some bytes into an encrypted record
type Encrypter interface {
	Encrypt(data []byte) (*api.MaybeEncryptedRecord, error)
}

type noopCrypter struct{}

func (n noopCrypter) Decrypt(e api.MaybeEncryptedRecord) ([]byte, error) {
	if e.Algorithm != n.Algorithm() {
		return nil, fmt.Errorf("record is encrypted")
	}
	return e.Data, nil
}

func (n noopCrypter) Encrypt(data []byte) (*api.MaybeEncryptedRecord, error) {
	return &api.MaybeEncryptedRecord{
		Algorithm: n.Algorithm(),
		Data:      data,
	}, nil
}

func (n noopCrypter) Algorithm() api.MaybeEncryptedRecord_Algorithm {
	return api.MaybeEncryptedRecord_NotEncrypted
}

// NoopCrypter is just a pass-through crypter - it does not actually encrypt or
// decrypt any data
var NoopCrypter = noopCrypter{}

// specificDecryptor represents a specific type of Decrypter, like NaclSecretbox or Fernet.
// It does not apply to a more general decrypter like MultiDecrypter.
type specificDecrypter interface {
	Decrypter
	Algorithm() api.MaybeEncryptedRecord_Algorithm
}

// MultiDecrypter is a decrypter that will attempt to decrypt with multiple decrypters.  It
// references them by algorithm, so that only the relevant decrypters are checked instead of
// every single one. The reason for multiple decrypters per algorithm is to support hitless
// encryption key rotation.
//
// For raft encryption for instance, during an encryption key rotation, it's possible to have
// some raft logs encrypted with the old key and some encrypted with the new key, so we need a
// decrypter that can decrypt both.
type MultiDecrypter struct {
	decrypters map[api.MaybeEncryptedRecord_Algorithm][]Decrypter
}

// Decrypt tries to decrypt using any decrypters that match the given algorithm.
func (m MultiDecrypter) Decrypt(r api.MaybeEncryptedRecord) ([]byte, error) {
	decrypters, ok := m.decrypters[r.Algorithm]
	if !ok {
		return nil, fmt.Errorf("cannot decrypt record encrypted using %s",
			api.MaybeEncryptedRecord_Algorithm_name[int32(r.Algorithm)])
	}
	var rerr error
	for _, d := range decrypters {
		result, err := d.Decrypt(r)
		if err == nil {
			return result, nil
		}
		rerr = err
	}
	return nil, rerr
}

// NewMultiDecrypter returns a new MultiDecrypter given multiple Decrypters.  If any of
// the Decrypters are also MultiDecrypters, they are flattened into a single map, but
// it does not deduplicate any decrypters.
// Note that if something is neither a MultiDecrypter nor a specificDecrypter, it is
// ignored.
func NewMultiDecrypter(decrypters ...Decrypter) MultiDecrypter {
	m := MultiDecrypter{decrypters: make(map[api.MaybeEncryptedRecord_Algorithm][]Decrypter)}
	for _, d := range decrypters {
		if md, ok := d.(MultiDecrypter); ok {
			for algo, dec := range md.decrypters {
				m.decrypters[algo] = append(m.decrypters[algo], dec...)
			}
		} else if sd, ok := d.(specificDecrypter); ok {
			m.decrypters[sd.Algorithm()] = append(m.decrypters[sd.Algorithm()], sd)
		}
	}
	return m
}

// Decrypt turns a slice of bytes serialized as an MaybeEncryptedRecord into a slice of plaintext bytes
func Decrypt(encryptd []byte, decrypter Decrypter) ([]byte, error) {
	if decrypter == nil {
		return nil, ErrCannotDecrypt{msg: "no decrypter specified"}
	}
	r := api.MaybeEncryptedRecord{}
	if err := proto.Unmarshal(encryptd, &r); err != nil {
		// nope, this wasn't marshalled as a MaybeEncryptedRecord
		return nil, ErrCannotDecrypt{msg: "unable to unmarshal as MaybeEncryptedRecord"}
	}
	plaintext, err := decrypter.Decrypt(r)
	if err != nil {
		return nil, ErrCannotDecrypt{msg: err.Error()}
	}
	return plaintext, nil
}

// Encrypt turns a slice of bytes into a serialized MaybeEncryptedRecord slice of bytes
func Encrypt(plaintext []byte, encrypter Encrypter) ([]byte, error) {
	if encrypter == nil {
		return nil, fmt.Errorf("no encrypter specified")
	}

	encryptedRecord, err := encrypter.Encrypt(plaintext)
	if err != nil {
		return nil, errors.Wrap(err, "unable to encrypt data")
	}

	data, err := proto.Marshal(encryptedRecord)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal as MaybeEncryptedRecord")
	}

	return data, nil
}

// Defaults returns a default encrypter and decrypter.  If the FIPS parameter is set to
// true, the only algorithm supported on both the encrypter and decrypter will be fernet.
func Defaults(key []byte, fips bool) (Encrypter, Decrypter) {
	f := NewFernet(key)
	if fips {
		return f, f
	}
	n := NewNACLSecretbox(key)
	return n, NewMultiDecrypter(n, f)
}

// GenerateSecretKey generates a secret key that can be used for encrypting data
// using this package
func GenerateSecretKey() []byte {
	secretData := make([]byte, naclSecretboxKeySize)
	if _, err := io.ReadFull(cryptorand.Reader, secretData); err != nil {
		// panic if we can't read random data
		panic(errors.Wrap(err, "failed to read random bytes"))
	}
	return secretData
}

// HumanReadableKey displays a secret key in a human readable way
func HumanReadableKey(key []byte) string {
	// base64-encode the key
	return humanReadablePrefix + base64.RawStdEncoding.EncodeToString(key)
}

// ParseHumanReadableKey returns a key as bytes from recognized serializations of
// said keys
func ParseHumanReadableKey(key string) ([]byte, error) {
	if !strings.HasPrefix(key, humanReadablePrefix) {
		return nil, fmt.Errorf("invalid key string")
	}
	keyBytes, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(key, humanReadablePrefix))
	if err != nil {
		return nil, fmt.Errorf("invalid key string")
	}
	return keyBytes, nil
}
