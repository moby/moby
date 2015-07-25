package signed

import (
	"github.com/endophage/gotuf/data"
)

// SigningService defines the necessary functions to determine
// if a user is able to sign with a key, and to perform signing.
type SigningService interface {
	// Sign takes a slice of keyIDs and a piece of data to sign
	// and returns a slice of signatures and an error
	Sign(keyIDs []string, data []byte) ([]data.Signature, error)
}

// KeyService provides management of keys locally. It will never
// accept or provide private keys. Communication between the KeyService
// and a SigningService happen behind the Create function.
type KeyService interface {
	// Create issues a new key pair and is responsible for loading
	// the private key into the appropriate signing service.
	// The role isn't currently used for anything, but it's here to support
	// future features
	Create(role string, algorithm data.KeyAlgorithm) (data.PublicKey, error)

	// GetKey retrieves the public key if present, otherwise it returns nil
	GetKey(keyID string) data.PublicKey

	// RemoveKey deletes the specified key
	RemoveKey(keyID string) error
}

// CryptoService defines a unified Signing and Key Service as this
// will be most useful for most applications.
type CryptoService interface {
	SigningService
	KeyService
}

// Verifier defines an interface for verfying signatures. An implementer
// of this interface should verify signatures for one and only one
// signing scheme.
type Verifier interface {
	Verify(key data.PublicKey, sig []byte, msg []byte) error
}
