package encryption

import (
	cryptorand "crypto/rand"
	"fmt"
	"io"

	"github.com/moby/swarmkit/v2/api"

	"golang.org/x/crypto/nacl/secretbox"
)

const naclSecretboxKeySize = 32
const naclSecretboxNonceSize = 24

// This provides the default implementation of an encrypter and decrypter, as well
// as the default KDF function.

// NACLSecretbox is an implementation of an encrypter/decrypter.  Encrypting
// generates random Nonces.
type NACLSecretbox struct {
	key [naclSecretboxKeySize]byte
}

// NewNACLSecretbox returns a new NACL secretbox encrypter/decrypter with the given key
func NewNACLSecretbox(key []byte) NACLSecretbox {
	secretbox := NACLSecretbox{}
	copy(secretbox.key[:], key)
	return secretbox
}

// Algorithm returns the type of algorithm this is (NACL Secretbox using XSalsa20 and Poly1305)
func (n NACLSecretbox) Algorithm() api.MaybeEncryptedRecord_Algorithm {
	return api.MaybeEncryptedRecord_NACLSecretboxSalsa20Poly1305
}

// Encrypt encrypts some bytes and returns an encrypted record
func (n NACLSecretbox) Encrypt(data []byte) (*api.MaybeEncryptedRecord, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(cryptorand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	// Seal's first argument is an "out", the data that the new encrypted message should be
	// appended to.  Since we don't want to append anything, we pass nil.
	encrypted := secretbox.Seal(nil, data, &nonce, &n.key)
	return &api.MaybeEncryptedRecord{
		Algorithm: n.Algorithm(),
		Data:      encrypted,
		Nonce:     nonce[:],
	}, nil
}

// Decrypt decrypts a MaybeEncryptedRecord and returns some bytes
func (n NACLSecretbox) Decrypt(record api.MaybeEncryptedRecord) ([]byte, error) {
	if record.Algorithm != n.Algorithm() {
		return nil, fmt.Errorf("not a NACL secretbox record")
	}
	if len(record.Nonce) != naclSecretboxNonceSize {
		return nil, fmt.Errorf("invalid nonce size for NACL secretbox: require 24, got %d", len(record.Nonce))
	}

	var decryptNonce [naclSecretboxNonceSize]byte
	copy(decryptNonce[:], record.Nonce[:naclSecretboxNonceSize])

	// Open's first argument is an "out", the data that the decrypted message should be
	// appended to.  Since we don't want to append anything, we pass nil.
	decrypted, ok := secretbox.Open(nil, record.Data, &decryptNonce, &n.key)
	if !ok {
		return nil, fmt.Errorf("no decryption key for record encrypted with %s", n.Algorithm())
	}
	return decrypted, nil
}
