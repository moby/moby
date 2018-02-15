// Package keyutils serves as a utility to parse, encrypt and decrypt
// PKCS#1 and PKCS#8 private keys based on current FIPS mode status,
// supporting only EC type keys. It always allows PKCS#8 private keys
// and disallow PKCS#1 private keys in FIPS-mode.
package keyutils

import (
	"crypto"
	cryptorand "crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/docker/swarmkit/ca/pkcs8"
)

var errFIPSUnsupportedKeyFormat = errors.New("unsupported key format due to FIPS compliance")

// FIPSEnvVar is the environment variable which stores FIPS mode state
const FIPSEnvVar = "GOFIPS"

// FIPSEnabled returns true when FIPS mode is enabled
func FIPSEnabled() bool {
	return os.Getenv(FIPSEnvVar) != ""
}

// IsPKCS8 returns true if the provided der bytes is encrypted/unencrypted PKCS#8 key
func IsPKCS8(derBytes []byte) bool {
	if _, err := x509.ParsePKCS8PrivateKey(derBytes); err == nil {
		return true
	}

	return pkcs8.IsEncryptedPEMBlock(&pem.Block{
		Type:    "PRIVATE KEY",
		Headers: nil,
		Bytes:   derBytes,
	})
}

// ParsePrivateKeyPEMWithPassword parses an encrypted or a decrypted PKCS#1 or PKCS#8 PEM to crypto.Signer.
// It returns an error in FIPS mode if PKCS#1 PEM bytes are passed.
func ParsePrivateKeyPEMWithPassword(pemBytes, password []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("Could not parse PEM")
	}

	if IsPKCS8(block.Bytes) {
		return pkcs8.ParsePrivateKeyPEMWithPassword(pemBytes, password)
	} else if FIPSEnabled() {
		return nil, errFIPSUnsupportedKeyFormat
	}

	return helpers.ParsePrivateKeyPEMWithPassword(pemBytes, password)
}

// IsEncryptedPEMBlock checks if a PKCS#1 or PKCS#8 PEM-block is encrypted or not
// It returns false in FIPS mode even if PKCS#1 is encrypted
func IsEncryptedPEMBlock(block *pem.Block) bool {
	return pkcs8.IsEncryptedPEMBlock(block) || (!FIPSEnabled() && x509.IsEncryptedPEMBlock(block))
}

// DecryptPEMBlock requires PKCS#1 or PKCS#8 PEM Block and password to decrypt and return unencrypted der []byte
// It returns an error in FIPS mode when PKCS#1 PEM Block is passed.
func DecryptPEMBlock(block *pem.Block, password []byte) ([]byte, error) {
	if IsPKCS8(block.Bytes) {
		return pkcs8.DecryptPEMBlock(block, password)
	} else if FIPSEnabled() {
		return nil, errFIPSUnsupportedKeyFormat
	}

	return x509.DecryptPEMBlock(block, password)
}

// EncryptPEMBlock takes DER-format bytes and password to return an encrypted PKCS#1 or PKCS#8 PEM-block
// It returns an error in FIPS mode when PKCS#1 PEM bytes are passed.
func EncryptPEMBlock(data, password []byte) (*pem.Block, error) {
	if IsPKCS8(data) {
		return pkcs8.EncryptPEMBlock(data, password)
	} else if FIPSEnabled() {
		return nil, errFIPSUnsupportedKeyFormat
	}

	cipherType := x509.PEMCipherAES256
	return x509.EncryptPEMBlock(cryptorand.Reader,
		"EC PRIVATE KEY",
		data,
		password,
		cipherType)
}
