package encryption

import (
	"fmt"

	"github.com/moby/swarmkit/v2/api"

	"github.com/fernet/fernet-go"
)

// Fernet wraps the `fernet` library as an implementation of encrypter/decrypter.
type Fernet struct {
	key fernet.Key
}

// NewFernet returns a new Fernet encrypter/decrypter with the given key
func NewFernet(key []byte) Fernet {
	frnt := Fernet{}
	copy(frnt.key[:], key)
	return frnt
}

// Algorithm returns the type of algorithm this is (Fernet, which uses AES128-CBC)
func (f Fernet) Algorithm() api.MaybeEncryptedRecord_Algorithm {
	return api.MaybeEncryptedRecord_FernetAES128CBC
}

// Encrypt encrypts some bytes and returns an encrypted record
func (f Fernet) Encrypt(data []byte) (*api.MaybeEncryptedRecord, error) {
	out, err := fernet.EncryptAndSign(data, &f.key)
	if err != nil {
		return nil, err
	}
	// fernet generates its own IVs, so nonce is empty
	return &api.MaybeEncryptedRecord{
		Algorithm: f.Algorithm(),
		Data:      out,
	}, nil
}

// Decrypt decrypts a MaybeEncryptedRecord and returns some bytes
func (f Fernet) Decrypt(record api.MaybeEncryptedRecord) ([]byte, error) {
	if record.Algorithm != f.Algorithm() {
		return nil, fmt.Errorf("record is not a Fernet message")
	}

	// -1 skips the TTL check, since we don't care about message expiry
	out := fernet.VerifyAndDecrypt(record.Data, -1, []*fernet.Key{&f.key})
	// VerifyandDecrypt returns a nil message if it can't be verified and decrypted
	if out == nil {
		return nil, fmt.Errorf("no decryption key for record encrypted with %s", f.Algorithm())
	}
	return out, nil
}
