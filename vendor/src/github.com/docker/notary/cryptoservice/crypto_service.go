package cryptoservice

import (
	"crypto/rand"
	"fmt"

	"crypto/x509"
	"encoding/pem"
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/utils"
)

var (
	// ErrNoValidPrivateKey is returned if a key being imported doesn't
	// look like a private key
	ErrNoValidPrivateKey = errors.New("no valid private key found")

	// ErrRootKeyNotEncrypted is returned if a root key being imported is
	// unencrypted
	ErrRootKeyNotEncrypted = errors.New("only encrypted root keys may be imported")
)

// CryptoService implements Sign and Create, holding a specific GUN and keystore to
// operate on
type CryptoService struct {
	keyStores []trustmanager.KeyStore
}

// NewCryptoService returns an instance of CryptoService
func NewCryptoService(keyStores ...trustmanager.KeyStore) *CryptoService {
	return &CryptoService{keyStores: keyStores}
}

// Create is used to generate keys for targets, snapshots and timestamps
func (cs *CryptoService) Create(role, gun, algorithm string) (data.PublicKey, error) {
	var privKey data.PrivateKey
	var err error

	switch algorithm {
	case data.RSAKey:
		privKey, err = utils.GenerateRSAKey(rand.Reader, notary.MinRSABitSize)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA key: %v", err)
		}
	case data.ECDSAKey:
		privKey, err = utils.GenerateECDSAKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate EC key: %v", err)
		}
	case data.ED25519Key:
		privKey, err = utils.GenerateED25519Key(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate ED25519 key: %v", err)
		}
	default:
		return nil, fmt.Errorf("private key type not supported for key generation: %s", algorithm)
	}
	logrus.Debugf("generated new %s key for role: %s and keyID: %s", algorithm, role, privKey.ID())

	// Store the private key into our keystore
	for _, ks := range cs.keyStores {
		err = ks.AddKey(trustmanager.KeyInfo{Role: role, Gun: gun}, privKey)
		if err == nil {
			return data.PublicKeyFromPrivate(privKey), nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to add key to filestore: %v", err)
	}

	return nil, fmt.Errorf("keystores would not accept new private keys for unknown reasons")
}

// GetPrivateKey returns a private key and role if present by ID.
func (cs *CryptoService) GetPrivateKey(keyID string) (k data.PrivateKey, role string, err error) {
	for _, ks := range cs.keyStores {
		if k, role, err = ks.GetKey(keyID); err == nil {
			return
		}
		switch err.(type) {
		case trustmanager.ErrPasswordInvalid, trustmanager.ErrAttemptsExceeded:
			return
		default:
			continue
		}
	}
	return // returns whatever the final values were
}

// GetKey returns a key by ID
func (cs *CryptoService) GetKey(keyID string) data.PublicKey {
	privKey, _, err := cs.GetPrivateKey(keyID)
	if err != nil {
		return nil
	}
	return data.PublicKeyFromPrivate(privKey)
}

// GetKeyInfo returns role and GUN info of a key by ID
func (cs *CryptoService) GetKeyInfo(keyID string) (trustmanager.KeyInfo, error) {
	for _, store := range cs.keyStores {
		if info, err := store.GetKeyInfo(keyID); err == nil {
			return info, nil
		}
	}
	return trustmanager.KeyInfo{}, fmt.Errorf("Could not find info for keyID %s", keyID)
}

// RemoveKey deletes a key by ID
func (cs *CryptoService) RemoveKey(keyID string) (err error) {
	for _, ks := range cs.keyStores {
		ks.RemoveKey(keyID)
	}
	return // returns whatever the final values were
}

// AddKey adds a private key to a specified role.
// The GUN is inferred from the cryptoservice itself for non-root roles
func (cs *CryptoService) AddKey(role, gun string, key data.PrivateKey) (err error) {
	// First check if this key already exists in any of our keystores
	for _, ks := range cs.keyStores {
		if keyInfo, err := ks.GetKeyInfo(key.ID()); err == nil {
			if keyInfo.Role != role {
				return fmt.Errorf("key with same ID already exists for role: %s", keyInfo.Role)
			}
			logrus.Debugf("key with same ID %s and role %s already exists", key.ID(), keyInfo.Role)
			return nil
		}
	}
	// If the key didn't exist in any of our keystores, add and return on the first successful keystore
	for _, ks := range cs.keyStores {
		// Try to add to this keystore, return if successful
		if err = ks.AddKey(trustmanager.KeyInfo{Role: role, Gun: gun}, key); err == nil {
			return nil
		}
	}
	return // returns whatever the final values were
}

// ListKeys returns a list of key IDs valid for the given role
func (cs *CryptoService) ListKeys(role string) []string {
	var res []string
	for _, ks := range cs.keyStores {
		for k, r := range ks.ListKeys() {
			if r.Role == role {
				res = append(res, k)
			}
		}
	}
	return res
}

// ListAllKeys returns a map of key IDs to role
func (cs *CryptoService) ListAllKeys() map[string]string {
	res := make(map[string]string)
	for _, ks := range cs.keyStores {
		for k, r := range ks.ListKeys() {
			res[k] = r.Role // keys are content addressed so don't care about overwrites
		}
	}
	return res
}

// CheckRootKeyIsEncrypted makes sure the root key is encrypted. We have
// internal assumptions that depend on this.
func CheckRootKeyIsEncrypted(pemBytes []byte) error {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return ErrNoValidPrivateKey
	}

	if !x509.IsEncryptedPEMBlock(block) {
		return ErrRootKeyNotEncrypted
	}

	return nil
}
