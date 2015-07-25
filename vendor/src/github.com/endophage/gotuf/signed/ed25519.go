package signed

import (
	"crypto/rand"
	"errors"

	"github.com/agl/ed25519"
	"github.com/endophage/gotuf/data"
)

// Ed25519 implements a simple in memory cryptosystem for ED25519 keys
type Ed25519 struct {
	keys map[string]data.PrivateKey
}

func NewEd25519() *Ed25519 {
	return &Ed25519{
		make(map[string]data.PrivateKey),
	}
}

// addKey allows you to add a private key
func (e *Ed25519) addKey(k data.PrivateKey) {
	e.keys[k.ID()] = k
}

func (e *Ed25519) RemoveKey(keyID string) error {
	delete(e.keys, keyID)
	return nil
}

func (e *Ed25519) Sign(keyIDs []string, toSign []byte) ([]data.Signature, error) {
	signatures := make([]data.Signature, 0, len(keyIDs))
	for _, kID := range keyIDs {
		priv := [ed25519.PrivateKeySize]byte{}
		copy(priv[:], e.keys[kID].Private())
		sig := ed25519.Sign(&priv, toSign)
		signatures = append(signatures, data.Signature{
			KeyID:     kID,
			Method:    data.EDDSASignature,
			Signature: sig[:],
		})
	}
	return signatures, nil

}

func (e *Ed25519) Create(role string, algorithm data.KeyAlgorithm) (data.PublicKey, error) {
	if algorithm != data.ED25519Key {
		return nil, errors.New("only ED25519 supported by this cryptoservice")
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	public := data.NewPublicKey(data.ED25519Key, pub[:])
	private := data.NewPrivateKey(data.ED25519Key, pub[:], priv[:])
	e.addKey(private)
	return public, nil
}

func (e *Ed25519) PublicKeys(keyIDs ...string) (map[string]data.PublicKey, error) {
	k := make(map[string]data.PublicKey)
	for _, kID := range keyIDs {
		if key, ok := e.keys[kID]; ok {
			k[kID] = data.PublicKeyFromPrivate(key)
		}
	}
	return k, nil
}

func (e *Ed25519) GetKey(keyID string) data.PublicKey {
	return data.PublicKeyFromPrivate(e.keys[keyID])
}
