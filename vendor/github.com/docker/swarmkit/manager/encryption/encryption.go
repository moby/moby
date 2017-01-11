package encryption

import (
	"crypto/rand"
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

// Defaults returns a default encrypter and decrypter
func Defaults(key []byte) (Encrypter, Decrypter) {
	n := NewNACLSecretbox(key)
	return n, n
}

// GenerateSecretKey generates a secret key that can be used for encrypting data
// using this package
func GenerateSecretKey() []byte {
	secretData := make([]byte, naclSecretboxKeySize)
	if _, err := io.ReadFull(rand.Reader, secretData); err != nil {
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
