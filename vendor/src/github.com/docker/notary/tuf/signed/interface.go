package signed

import (
	"github.com/docker/notary/tuf/data"
	"io"
)

// KeyService provides management of keys locally. It will never
// accept or provide private keys. Communication between the KeyService
// and a SigningService happen behind the Create function.
type KeyService interface {
	// Create issues a new key pair and is responsible for loading
	// the private key into the appropriate signing service.
	// The role isn't currently used for anything, but it's here to support
	// future features
	Create(role, algorithm string) (data.PublicKey, error)

	// GetKey retrieves the public key if present, otherwise it returns nil
	GetKey(keyID string) data.PublicKey

	// GetPrivateKey retrieves the private key and role if present, otherwise
	// it returns nil
	GetPrivateKey(keyID string) (data.PrivateKey, string, error)

	// RemoveKey deletes the specified key
	RemoveKey(keyID string) error

	// ListKeys returns a list of key IDs for the role
	ListKeys(role string) []string

	// ListAllKeys returns a map of all available signing key IDs to role
	ListAllKeys() map[string]string

	// ImportRootKey imports a root key to the highest priority keystore associated with
	// the cryptoservice
	ImportRootKey(source io.Reader) error
}

// CryptoService is deprecated and all instances of its use should be
// replaced with KeyService
type CryptoService interface {
	KeyService
}

// Verifier defines an interface for verfying signatures. An implementer
// of this interface should verify signatures for one and only one
// signing scheme.
type Verifier interface {
	Verify(key data.PublicKey, sig []byte, msg []byte) error
}
