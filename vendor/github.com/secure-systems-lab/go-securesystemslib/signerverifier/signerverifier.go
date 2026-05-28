package signerverifier

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
)

var KeyIDHashAlgorithms = []string{"sha256", "sha512"}

var (
	ErrNotPrivateKey               = errors.New("loaded key is not a private key")
	ErrSignatureVerificationFailed = errors.New("failed to verify signature")
	ErrUnknownKeyType              = errors.New("unknown key type")
	ErrInvalidThreshold            = errors.New("threshold is either less than 1 or greater than number of provided public keys")
	ErrInvalidKey                  = errors.New("key object has no value")
	ErrInvalidPEM                  = errors.New("unable to parse PEM block")
)

const (
	PublicKeyPEM  = "PUBLIC KEY"
	PrivateKeyPEM = "PRIVATE KEY"
)

type SSLibKey struct {
	KeyIDHashAlgorithms []string `json:"keyid_hash_algorithms"`
	KeyType             string   `json:"keytype"`
	KeyVal              KeyVal   `json:"keyval"`
	Scheme              string   `json:"scheme"`
	KeyID               string   `json:"keyid"`
}

type KeyVal struct {
	Private     string `json:"private,omitempty"`
	Public      string `json:"public,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	Identity    string `json:"identity,omitempty"`
	Issuer      string `json:"issuer,omitempty"`
}

// LoadKey returns an SSLibKey object when provided a PEM encoded key.
// Currently, RSA, ED25519, and ECDSA keys are supported.
func LoadKey(keyBytes []byte) (*SSLibKey, error) {
	pemBlock, rawKey, err := decodeAndParsePEM(keyBytes)
	if err != nil {
		return nil, err
	}

	var key *SSLibKey
	switch k := rawKey.(type) {
	case *rsa.PublicKey:
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             RSAKeyType,
			KeyVal: KeyVal{
				Public: strings.TrimSpace(string(generatePEMBlock(pubKeyBytes, PublicKeyPEM))),
			},
			Scheme: RSAKeyScheme,
		}

	case *rsa.PrivateKey:
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(k.Public())
		if err != nil {
			return nil, err
		}
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             RSAKeyType,
			KeyVal: KeyVal{
				Public:  strings.TrimSpace(string(generatePEMBlock(pubKeyBytes, PublicKeyPEM))),
				Private: strings.TrimSpace(string(generatePEMBlock(pemBlock.Bytes, pemBlock.Type))),
			},
			Scheme: RSAKeyScheme,
		}

	case ed25519.PublicKey:
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             ED25519KeyType,
			KeyVal: KeyVal{
				Public: strings.TrimSpace(hex.EncodeToString(k)),
			},
			Scheme: ED25519KeyType,
		}

	case ed25519.PrivateKey:
		pubKeyBytes := k.Public()
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             ED25519KeyType,
			KeyVal: KeyVal{
				Public:  strings.TrimSpace(hex.EncodeToString(pubKeyBytes.(ed25519.PublicKey))),
				Private: strings.TrimSpace(hex.EncodeToString(k)),
			},
			Scheme: ED25519KeyType,
		}

	case *ecdsa.PublicKey:
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             ECDSAKeyType,
			KeyVal: KeyVal{
				Public: strings.TrimSpace(string(generatePEMBlock(pubKeyBytes, PublicKeyPEM))),
			},
			Scheme: ECDSAKeyScheme,
		}

	case *ecdsa.PrivateKey:
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(k.Public())
		if err != nil {
			return nil, err
		}
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             ECDSAKeyType,
			KeyVal: KeyVal{
				Public:  strings.TrimSpace(string(generatePEMBlock(pubKeyBytes, PublicKeyPEM))),
				Private: strings.TrimSpace(string(generatePEMBlock(pemBlock.Bytes, PrivateKeyPEM))),
			},
			Scheme: ECDSAKeyScheme,
		}

	default:
		return nil, ErrUnknownKeyType
	}

	keyID, err := calculateKeyID(key)
	if err != nil {
		return nil, err
	}
	key.KeyID = keyID

	return key, nil
}
