package cryptoservice

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/agl/ed25519"
	"github.com/docker/notary/trustmanager"
	"github.com/endophage/gotuf/data"
)

const (
	rsaKeySize = 2048 // Used for snapshots and targets keys
)

// CryptoService implements Sign and Create, holding a specific GUN and keystore to
// operate on
type CryptoService struct {
	gun      string
	keyStore trustmanager.KeyStore
}

// NewCryptoService returns an instance of CryptoService
func NewCryptoService(gun string, keyStore trustmanager.KeyStore) *CryptoService {
	return &CryptoService{gun: gun, keyStore: keyStore}
}

// Create is used to generate keys for targets, snapshots and timestamps
func (ccs *CryptoService) Create(role string, algorithm data.KeyAlgorithm) (data.PublicKey, error) {
	var privKey data.PrivateKey
	var err error

	switch algorithm {
	case data.RSAKey:
		privKey, err = trustmanager.GenerateRSAKey(rand.Reader, rsaKeySize)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA key: %v", err)
		}
	case data.ECDSAKey:
		privKey, err = trustmanager.GenerateECDSAKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate EC key: %v", err)
		}
	case data.ED25519Key:
		privKey, err = trustmanager.GenerateED25519Key(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate ED25519 key: %v", err)
		}
	default:
		return nil, fmt.Errorf("private key type not supported for key generation: %s", algorithm)
	}
	logrus.Debugf("generated new %s key for role: %s and keyID: %s", algorithm, role, privKey.ID())

	// Store the private key into our keystore with the name being: /GUN/ID.key with an alias of role
	err = ccs.keyStore.AddKey(filepath.Join(ccs.gun, privKey.ID()), role, privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to add key to filestore: %v", err)
	}
	return data.PublicKeyFromPrivate(privKey), nil
}

// GetKey returns a key by ID
func (ccs *CryptoService) GetKey(keyID string) data.PublicKey {
	key, _, err := ccs.keyStore.GetKey(keyID)
	if err != nil {
		return nil
	}
	return data.PublicKeyFromPrivate(key)
}

// RemoveKey deletes a key by ID
func (ccs *CryptoService) RemoveKey(keyID string) error {
	return ccs.keyStore.RemoveKey(keyID)
}

// Sign returns the signatures for the payload with a set of keyIDs. It ignores
// errors to sign and expects the called to validate if the number of returned
// signatures is adequate.
func (ccs *CryptoService) Sign(keyIDs []string, payload []byte) ([]data.Signature, error) {
	signatures := make([]data.Signature, 0, len(keyIDs))
	for _, keyid := range keyIDs {
		// ccs.gun will be empty if this is the root key
		keyName := filepath.Join(ccs.gun, keyid)

		var privKey data.PrivateKey
		var err error

		privKey, _, err = ccs.keyStore.GetKey(keyName)
		if err != nil {
			logrus.Debugf("error attempting to retrieve key ID: %s, %v", keyid, err)
			return nil, err
		}

		algorithm := privKey.Algorithm()
		var sigAlgorithm data.SigAlgorithm
		var sig []byte

		switch algorithm {
		case data.RSAKey:
			sig, err = rsaSign(privKey, payload)
			sigAlgorithm = data.RSAPSSSignature
		case data.ECDSAKey:
			sig, err = ecdsaSign(privKey, payload)
			sigAlgorithm = data.ECDSASignature
		case data.ED25519Key:
			// ED25519 does not operate on a SHA256 hash
			sig, err = ed25519Sign(privKey, payload)
			sigAlgorithm = data.EDDSASignature
		}
		if err != nil {
			logrus.Debugf("ignoring error attempting to %s sign with keyID: %s, %v", algorithm, keyid, err)
			return nil, err
		}

		logrus.Debugf("appending %s signature with Key ID: %s", algorithm, keyid)

		// Append signatures to result array
		signatures = append(signatures, data.Signature{
			KeyID:     keyid,
			Method:    sigAlgorithm,
			Signature: sig[:],
		})
	}

	return signatures, nil
}

func rsaSign(privKey data.PrivateKey, message []byte) ([]byte, error) {
	if privKey.Algorithm() != data.RSAKey {
		return nil, fmt.Errorf("private key type not supported: %s", privKey.Algorithm())
	}

	hashed := sha256.Sum256(message)

	// Create an rsa.PrivateKey out of the private key bytes
	rsaPrivKey, err := x509.ParsePKCS1PrivateKey(privKey.Private())
	if err != nil {
		return nil, err
	}

	// Use the RSA key to RSASSA-PSS sign the data
	sig, err := rsa.SignPSS(rand.Reader, rsaPrivKey, crypto.SHA256, hashed[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	if err != nil {
		return nil, err
	}

	return sig, nil
}

func ecdsaSign(privKey data.PrivateKey, message []byte) ([]byte, error) {
	if privKey.Algorithm() != data.ECDSAKey {
		return nil, fmt.Errorf("private key type not supported: %s", privKey.Algorithm())
	}

	hashed := sha256.Sum256(message)

	// Create an ecdsa.PrivateKey out of the private key bytes
	ecdsaPrivKey, err := x509.ParseECPrivateKey(privKey.Private())
	if err != nil {
		return nil, err
	}

	// Use the ECDSA key to sign the data
	r, s, err := ecdsa.Sign(rand.Reader, ecdsaPrivKey, hashed[:])
	if err != nil {
		return nil, err
	}

	rBytes, sBytes := r.Bytes(), s.Bytes()
	octetLength := (ecdsaPrivKey.Params().BitSize + 7) >> 3

	// MUST include leading zeros in the output
	rBuf := make([]byte, octetLength-len(rBytes), octetLength)
	sBuf := make([]byte, octetLength-len(sBytes), octetLength)

	rBuf = append(rBuf, rBytes...)
	sBuf = append(sBuf, sBytes...)

	return append(rBuf, sBuf...), nil
}

func ed25519Sign(privKey data.PrivateKey, message []byte) ([]byte, error) {
	if privKey.Algorithm() != data.ED25519Key {
		return nil, fmt.Errorf("private key type not supported: %s", privKey.Algorithm())
	}

	priv := [ed25519.PrivateKeySize]byte{}
	copy(priv[:], privKey.Private()[ed25519.PublicKeySize:])
	sig := ed25519.Sign(&priv, message)

	return sig[:], nil
}
