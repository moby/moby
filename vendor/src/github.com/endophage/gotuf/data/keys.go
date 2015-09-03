package data

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Sirupsen/logrus"
	"github.com/jfrazelle/go/canonical/json"
)

type Key interface {
	ID() string
	Algorithm() KeyAlgorithm
	Public() []byte
}

type PublicKey interface {
	Key
}

type PrivateKey interface {
	Key

	Private() []byte
}

type KeyPair struct {
	Public  []byte `json:"public"`
	Private []byte `json:"private"`
}

// TUFKey is the structure used for both public and private keys in TUF.
// Normally it would make sense to use a different structures for public and
// private keys, but that would change the key ID algorithm (since the canonical
// JSON would be different). This structure should normally be accessed through
// the PublicKey or PrivateKey interfaces.
type TUFKey struct {
	id    string       `json:"-"`
	Type  KeyAlgorithm `json:"keytype"`
	Value KeyPair      `json:"keyval"`
}

func NewPrivateKey(algorithm KeyAlgorithm, public, private []byte) *TUFKey {
	return &TUFKey{
		Type: algorithm,
		Value: KeyPair{
			Public:  public,
			Private: private,
		},
	}
}

func (k TUFKey) Algorithm() KeyAlgorithm {
	return k.Type
}

func (k *TUFKey) ID() string {
	if k.id == "" {
		pubK := NewPublicKey(k.Algorithm(), k.Public())
		data, err := json.MarshalCanonical(&pubK)
		if err != nil {
			logrus.Error("Error generating key ID:", err)
		}
		digest := sha256.Sum256(data)
		k.id = hex.EncodeToString(digest[:])
	}
	return k.id
}

func (k TUFKey) Public() []byte {
	return k.Value.Public
}

func (k *TUFKey) Private() []byte {
	return k.Value.Private
}

func NewPublicKey(algorithm KeyAlgorithm, public []byte) PublicKey {
	return &TUFKey{
		Type: algorithm,
		Value: KeyPair{
			Public:  public,
			Private: nil,
		},
	}
}

func PublicKeyFromPrivate(pk PrivateKey) PublicKey {
	return &TUFKey{
		Type: pk.Algorithm(),
		Value: KeyPair{
			Public:  pk.Public(),
			Private: nil,
		},
	}
}
