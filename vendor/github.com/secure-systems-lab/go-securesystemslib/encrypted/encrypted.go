// Package encrypted provides a simple, secure system for encrypting data
// symmetrically with a passphrase.
//
// It uses scrypt derive a key from the passphrase and the NaCl secret box
// cipher for authenticated encryption.
package encrypted

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/scrypt"
)

const saltSize = 32

const (
	boxKeySize   = 32
	boxNonceSize = 24
)

// KDFParameterStrength defines the KDF parameter strength level to be used for
// encryption key derivation.
type KDFParameterStrength uint8

const (
	// Legacy defines legacy scrypt parameters (N:2^15, r:8, p:1)
	Legacy KDFParameterStrength = iota + 1
	// Standard defines standard scrypt parameters which is focusing 100ms of computation (N:2^16, r:8, p:1)
	Standard
	// OWASP defines OWASP recommended scrypt parameters (N:2^17, r:8, p:1)
	OWASP
)

var (
	// legacyParams represents old scrypt derivation parameters for backward
	// compatibility.
	legacyParams = scryptParams{
		N: 32768, // 2^15
		R: 8,
		P: 1,
	}

	// standardParams defines scrypt parameters based on the scrypt creator
	// recommendation to limit key derivation in time boxed to 100ms.
	standardParams = scryptParams{
		N: 65536, // 2^16
		R: 8,
		P: 1,
	}

	// owaspParams defines scrypt parameters recommended by OWASP
	owaspParams = scryptParams{
		N: 131072, // 2^17
		R: 8,
		P: 1,
	}

	// defaultParams defines scrypt parameters which will be used to generate a
	// new key.
	defaultParams = standardParams
)

const (
	nameScrypt    = "scrypt"
	nameSecretBox = "nacl/secretbox"
)

type data struct {
	KDF        scryptKDF       `json:"kdf"`
	Cipher     secretBoxCipher `json:"cipher"`
	Ciphertext []byte          `json:"ciphertext"`
}

type scryptParams struct {
	N int `json:"N"`
	R int `json:"r"`
	P int `json:"p"`
}

func (sp *scryptParams) Equal(in *scryptParams) bool {
	return in != nil && sp.N == in.N && sp.P == in.P && sp.R == in.R
}

func newScryptKDF(level KDFParameterStrength) (scryptKDF, error) {
	salt := make([]byte, saltSize)
	if err := fillRandom(salt); err != nil {
		return scryptKDF{}, fmt.Errorf("unable to generate a random salt: %w", err)
	}

	var params scryptParams
	switch level {
	case Legacy:
		params = legacyParams
	case Standard:
		params = standardParams
	case OWASP:
		params = owaspParams
	default:
		// Fallback to default parameters
		params = defaultParams
	}

	return scryptKDF{
		Name:   nameScrypt,
		Params: params,
		Salt:   salt,
	}, nil
}

type scryptKDF struct {
	Name   string       `json:"name"`
	Params scryptParams `json:"params"`
	Salt   []byte       `json:"salt"`
}

func (s *scryptKDF) Key(passphrase []byte) ([]byte, error) {
	return scrypt.Key(passphrase, s.Salt, s.Params.N, s.Params.R, s.Params.P, boxKeySize)
}

// CheckParams checks that the encoded KDF parameters are what we expect them to
// be. If we do not do this, an attacker could cause a DoS by tampering with
// them.
func (s *scryptKDF) CheckParams() error {
	switch {
	case legacyParams.Equal(&s.Params):
	case standardParams.Equal(&s.Params):
	case owaspParams.Equal(&s.Params):
	default:
		return errors.New("unsupported scrypt parameters")
	}

	return nil
}

func newSecretBoxCipher() (secretBoxCipher, error) {
	nonce := make([]byte, boxNonceSize)
	if err := fillRandom(nonce); err != nil {
		return secretBoxCipher{}, err
	}
	return secretBoxCipher{
		Name:  nameSecretBox,
		Nonce: nonce,
	}, nil
}

type secretBoxCipher struct {
	Name  string `json:"name"`
	Nonce []byte `json:"nonce"`

	encrypted bool
}

func (s *secretBoxCipher) Encrypt(plaintext, key []byte) []byte {
	var keyBytes [boxKeySize]byte
	var nonceBytes [boxNonceSize]byte

	if len(key) != len(keyBytes) {
		panic("incorrect key size")
	}
	if len(s.Nonce) != len(nonceBytes) {
		panic("incorrect nonce size")
	}

	copy(keyBytes[:], key)
	copy(nonceBytes[:], s.Nonce)

	// ensure that we don't re-use nonces
	if s.encrypted {
		panic("Encrypt must only be called once for each cipher instance")
	}
	s.encrypted = true

	return secretbox.Seal(nil, plaintext, &nonceBytes, &keyBytes)
}

func (s *secretBoxCipher) Decrypt(ciphertext, key []byte) ([]byte, error) {
	var keyBytes [boxKeySize]byte
	var nonceBytes [boxNonceSize]byte

	if len(key) != len(keyBytes) {
		panic("incorrect key size")
	}
	if len(s.Nonce) != len(nonceBytes) {
		// return an error instead of panicking since the nonce is user input
		return nil, errors.New("encrypted: incorrect nonce size")
	}

	copy(keyBytes[:], key)
	copy(nonceBytes[:], s.Nonce)

	res, ok := secretbox.Open(nil, ciphertext, &nonceBytes, &keyBytes)
	if !ok {
		return nil, errors.New("encrypted: decryption failed")
	}
	return res, nil
}

// Encrypt takes a passphrase and plaintext, and returns a JSON object
// containing ciphertext and the details necessary to decrypt it.
func Encrypt(plaintext, passphrase []byte) ([]byte, error) {
	return EncryptWithCustomKDFParameters(plaintext, passphrase, Standard)
}

// EncryptWithCustomKDFParameters takes a passphrase, the plaintext and a KDF
// parameter level (Legacy, Standard, or OWASP), and returns a JSON object
// containing ciphertext and the details necessary to decrypt it.
func EncryptWithCustomKDFParameters(plaintext, passphrase []byte, kdfLevel KDFParameterStrength) ([]byte, error) {
	k, err := newScryptKDF(kdfLevel)
	if err != nil {
		return nil, err
	}
	key, err := k.Key(passphrase)
	if err != nil {
		return nil, err
	}

	c, err := newSecretBoxCipher()
	if err != nil {
		return nil, err
	}

	data := &data{
		KDF:    k,
		Cipher: c,
	}
	data.Ciphertext = c.Encrypt(plaintext, key)

	return json.Marshal(data)
}

// Marshal encrypts the JSON encoding of v using passphrase.
func Marshal(v interface{}, passphrase []byte) ([]byte, error) {
	return MarshalWithCustomKDFParameters(v, passphrase, Standard)
}

// MarshalWithCustomKDFParameters encrypts the JSON encoding of v using passphrase.
func MarshalWithCustomKDFParameters(v interface{}, passphrase []byte, kdfLevel KDFParameterStrength) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return nil, err
	}
	return EncryptWithCustomKDFParameters(data, passphrase, kdfLevel)
}

// Decrypt takes a JSON-encoded ciphertext object encrypted using Encrypt and
// tries to decrypt it using passphrase. If successful, it returns the
// plaintext.
func Decrypt(ciphertext, passphrase []byte) ([]byte, error) {
	data := &data{}
	if err := json.Unmarshal(ciphertext, data); err != nil {
		return nil, err
	}

	if data.KDF.Name != nameScrypt {
		return nil, fmt.Errorf("encrypted: unknown kdf name %q", data.KDF.Name)
	}
	if data.Cipher.Name != nameSecretBox {
		return nil, fmt.Errorf("encrypted: unknown cipher name %q", data.Cipher.Name)
	}
	if err := data.KDF.CheckParams(); err != nil {
		return nil, err
	}

	key, err := data.KDF.Key(passphrase)
	if err != nil {
		return nil, err
	}

	return data.Cipher.Decrypt(data.Ciphertext, key)
}

// Unmarshal decrypts the data using passphrase and unmarshals the resulting
// plaintext into the value pointed to by v.
func Unmarshal(data []byte, v interface{}, passphrase []byte) error {
	decrypted, err := Decrypt(data, passphrase)
	if err != nil {
		return err
	}
	return json.Unmarshal(decrypted, v)
}

func fillRandom(b []byte) error {
	_, err := io.ReadFull(rand.Reader, b)
	return err
}
