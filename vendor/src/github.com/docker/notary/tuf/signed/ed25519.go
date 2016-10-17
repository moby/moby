package signed

import (
	"crypto/rand"
	"errors"

	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/utils"
)

type edCryptoKey struct {
	role    string
	privKey data.PrivateKey
}

// Ed25519 implements a simple in memory cryptosystem for ED25519 keys
type Ed25519 struct {
	keys map[string]edCryptoKey
}

// NewEd25519 initializes a new empty Ed25519 CryptoService that operates
// entirely in memory
func NewEd25519() *Ed25519 {
	return &Ed25519{
		make(map[string]edCryptoKey),
	}
}

// AddKey allows you to add a private key
func (e *Ed25519) AddKey(role, gun string, k data.PrivateKey) error {
	e.addKey(role, k)
	return nil
}

// addKey allows you to add a private key
func (e *Ed25519) addKey(role string, k data.PrivateKey) {
	e.keys[k.ID()] = edCryptoKey{
		role:    role,
		privKey: k,
	}
}

// RemoveKey deletes a key from the signer
func (e *Ed25519) RemoveKey(keyID string) error {
	delete(e.keys, keyID)
	return nil
}

// ListKeys returns the list of keys IDs for the role
func (e *Ed25519) ListKeys(role string) []string {
	keyIDs := make([]string, 0, len(e.keys))
	for id, edCryptoKey := range e.keys {
		if edCryptoKey.role == role {
			keyIDs = append(keyIDs, id)
		}
	}
	return keyIDs
}

// ListAllKeys returns the map of keys IDs to role
func (e *Ed25519) ListAllKeys() map[string]string {
	keys := make(map[string]string)
	for id, edKey := range e.keys {
		keys[id] = edKey.role
	}
	return keys
}

// Create generates a new key and returns the public part
func (e *Ed25519) Create(role, gun, algorithm string) (data.PublicKey, error) {
	if algorithm != data.ED25519Key {
		return nil, errors.New("only ED25519 supported by this cryptoservice")
	}

	private, err := utils.GenerateED25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}

	e.addKey(role, private)
	return data.PublicKeyFromPrivate(private), nil
}

// PublicKeys returns a map of public keys for the ids provided, when those IDs are found
// in the store.
func (e *Ed25519) PublicKeys(keyIDs ...string) (map[string]data.PublicKey, error) {
	k := make(map[string]data.PublicKey)
	for _, keyID := range keyIDs {
		if edKey, ok := e.keys[keyID]; ok {
			k[keyID] = data.PublicKeyFromPrivate(edKey.privKey)
		}
	}
	return k, nil
}

// GetKey returns a single public key based on the ID
func (e *Ed25519) GetKey(keyID string) data.PublicKey {
	if privKey, _, err := e.GetPrivateKey(keyID); err == nil {
		return data.PublicKeyFromPrivate(privKey)
	}
	return nil
}

// GetPrivateKey returns a single private key and role if present, based on the ID
func (e *Ed25519) GetPrivateKey(keyID string) (data.PrivateKey, string, error) {
	if k, ok := e.keys[keyID]; ok {
		return k.privKey, k.role, nil
	}
	return nil, "", trustmanager.ErrKeyNotFound{KeyID: keyID}
}
