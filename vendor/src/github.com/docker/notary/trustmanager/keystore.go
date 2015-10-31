package trustmanager

import (
	"fmt"

	"github.com/docker/notary/tuf/data"
)

// ErrAttemptsExceeded is returned when too many attempts have been made to decrypt a key
type ErrAttemptsExceeded struct{}

// ErrAttemptsExceeded is returned when too many attempts have been made to decrypt a key
func (err ErrAttemptsExceeded) Error() string {
	return "maximum number of passphrase attempts exceeded"
}

// ErrPasswordInvalid is returned when signing fails. It could also mean the signing
// key file was corrupted, but we have no way to distinguish.
type ErrPasswordInvalid struct{}

// ErrPasswordInvalid is returned when signing fails. It could also mean the signing
// key file was corrupted, but we have no way to distinguish.
func (err ErrPasswordInvalid) Error() string {
	return "password invalid, operation has failed."
}

// ErrKeyNotFound is returned when the keystore fails to retrieve a specific key.
type ErrKeyNotFound struct {
	KeyID string
}

// ErrKeyNotFound is returned when the keystore fails to retrieve a specific key.
func (err ErrKeyNotFound) Error() string {
	return fmt.Sprintf("signing key not found: %s", err.KeyID)
}

const (
	keyExtension = "key"
)

// KeyStore is a generic interface for private key storage
type KeyStore interface {
	// Add Key adds a key to the KeyStore, and if the key already exists,
	// succeeds.  Otherwise, returns an error if it cannot add.
	AddKey(name, alias string, privKey data.PrivateKey) error
	GetKey(name string) (data.PrivateKey, string, error)
	ListKeys() map[string]string
	RemoveKey(name string) error
	ExportKey(name string) ([]byte, error)
	ImportKey(pemBytes []byte, alias string) error
	Name() string
}

type cachedKey struct {
	alias string
	key   data.PrivateKey
}
