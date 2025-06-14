package signerverifier

import (
	"errors"
)

var KeyIDHashAlgorithms = []string{"sha256", "sha512"}

var (
	ErrNotPrivateKey               = errors.New("loaded key is not a private key")
	ErrSignatureVerificationFailed = errors.New("failed to verify signature")
	ErrUnknownKeyType              = errors.New("unknown key type")
	ErrInvalidThreshold            = errors.New("threshold is either less than 1 or greater than number of provided public keys")
	ErrInvalidKey                  = errors.New("key object has no value")
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
	Public      string `json:"public"`
	Certificate string `json:"certificate,omitempty"`
}
