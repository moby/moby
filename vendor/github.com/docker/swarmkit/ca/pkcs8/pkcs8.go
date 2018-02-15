// Package pkcs8 implements functions to encrypt, decrypt, parse and to convert
// EC private keys to PKCS#8 format. However this package is hard forked from
// https://github.com/youmark/pkcs8 and modified function signatures to match
// signatures of crypto/x509 and cloudflare/cfssl/helpers to simplify package
// swapping. License for original package is as follow:
//
// The MIT License (MIT)
//
// Copyright (c) 2014 youmark
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package pkcs8

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/asn1"
	"encoding/pem"
	"errors"

	"github.com/cloudflare/cfssl/helpers/derhelpers"
	"golang.org/x/crypto/pbkdf2"
)

// Copy from crypto/x509
var (
	oidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
)

// Unencrypted PKCS#8
var (
	oidPKCS5PBKDF2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidPBES2       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidAES256CBC   = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
)

type ecPrivateKey struct {
	Version       int
	PrivateKey    []byte
	NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
	PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
}

type privateKeyInfo struct {
	Version             int
	PrivateKeyAlgorithm []asn1.ObjectIdentifier
	PrivateKey          []byte
}

// Encrypted PKCS8
type pbkdf2Params struct {
	Salt           []byte
	IterationCount int
}

type pbkdf2Algorithms struct {
	IDPBKDF2     asn1.ObjectIdentifier
	PBKDF2Params pbkdf2Params
}

type pbkdf2Encs struct {
	EncryAlgo asn1.ObjectIdentifier
	IV        []byte
}

type pbes2Params struct {
	KeyDerivationFunc pbkdf2Algorithms
	EncryptionScheme  pbkdf2Encs
}

type pbes2Algorithms struct {
	IDPBES2     asn1.ObjectIdentifier
	PBES2Params pbes2Params
}

type encryptedPrivateKeyInfo struct {
	EncryptionAlgorithm pbes2Algorithms
	EncryptedData       []byte
}

// ParsePrivateKeyPEMWithPassword parses an encrypted or a decrypted PKCS#8 PEM to crypto.signer
func ParsePrivateKeyPEMWithPassword(pemBytes, password []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid pem file")
	}

	var (
		der []byte
		err error
	)
	der = block.Bytes

	if ok := IsEncryptedPEMBlock(block); ok {
		der, err = DecryptPEMBlock(block, password)
		if err != nil {
			return nil, err
		}
	}

	return derhelpers.ParsePrivateKeyDER(der)
}

// IsEncryptedPEMBlock checks if a PKCS#8 PEM-block is encrypted or not
func IsEncryptedPEMBlock(block *pem.Block) bool {
	der := block.Bytes

	var privKey encryptedPrivateKeyInfo
	if _, err := asn1.Unmarshal(der, &privKey); err != nil {
		return false
	}

	return true
}

// DecryptPEMBlock requires PKCS#8 PEM Block and password to decrypt and return unencrypted der []byte
func DecryptPEMBlock(block *pem.Block, password []byte) ([]byte, error) {
	der := block.Bytes

	var privKey encryptedPrivateKeyInfo
	if _, err := asn1.Unmarshal(der, &privKey); err != nil {
		return nil, errors.New("pkcs8: only PKCS #5 v2.0 supported")
	}

	if !privKey.EncryptionAlgorithm.IDPBES2.Equal(oidPBES2) {
		return nil, errors.New("pkcs8: only PBES2 supported")
	}

	if !privKey.EncryptionAlgorithm.PBES2Params.KeyDerivationFunc.IDPBKDF2.Equal(oidPKCS5PBKDF2) {
		return nil, errors.New("pkcs8: only PBKDF2 supported")
	}

	encParam := privKey.EncryptionAlgorithm.PBES2Params.EncryptionScheme
	kdfParam := privKey.EncryptionAlgorithm.PBES2Params.KeyDerivationFunc.PBKDF2Params

	switch {
	case encParam.EncryAlgo.Equal(oidAES256CBC):
		iv := encParam.IV
		salt := kdfParam.Salt
		iter := kdfParam.IterationCount

		encryptedKey := privKey.EncryptedData
		symkey := pbkdf2.Key(password, salt, iter, 32, sha1.New)
		block, err := aes.NewCipher(symkey)
		if err != nil {
			return nil, err
		}
		mode := cipher.NewCBCDecrypter(block, iv)
		mode.CryptBlocks(encryptedKey, encryptedKey)

		if _, err := derhelpers.ParsePrivateKeyDER(encryptedKey); err != nil {
			return nil, errors.New("pkcs8: incorrect password")
		}

		// Remove padding from key as it might be used to encode to memory as pem
		keyLen := len(encryptedKey)
		padLen := int(encryptedKey[keyLen-1])
		if padLen > keyLen || padLen > aes.BlockSize {
			return nil, errors.New("pkcs8: invalid padding size")
		}
		encryptedKey = encryptedKey[:keyLen-padLen]

		return encryptedKey, nil
	default:
		return nil, errors.New("pkcs8: only AES-256-CBC supported")
	}
}

func encryptPrivateKey(pkey, password []byte) ([]byte, error) {
	// Calculate key from password based on PKCS5 algorithm
	// Use 8 byte salt, 16 byte IV, and 2048 iteration
	iter := 2048
	salt := make([]byte, 8)
	iv := make([]byte, 16)

	if _, err := rand.Reader.Read(salt); err != nil {
		return nil, err
	}

	if _, err := rand.Reader.Read(iv); err != nil {
		return nil, err
	}

	key := pbkdf2.Key(password, salt, iter, 32, sha1.New)

	// Use AES256-CBC mode, pad plaintext with PKCS5 padding scheme
	n := len(pkey)
	padLen := aes.BlockSize - n%aes.BlockSize
	if padLen > 0 {
		padValue := []byte{byte(padLen)}
		padding := bytes.Repeat(padValue, padLen)
		pkey = append(pkey, padding...)
	}

	encryptedKey := make([]byte, len(pkey))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encryptedKey, pkey)

	pbkdf2algo := pbkdf2Algorithms{oidPKCS5PBKDF2, pbkdf2Params{salt, iter}}
	pbkdf2encs := pbkdf2Encs{oidAES256CBC, iv}
	pbes2algo := pbes2Algorithms{oidPBES2, pbes2Params{pbkdf2algo, pbkdf2encs}}

	encryptedPkey := encryptedPrivateKeyInfo{pbes2algo, encryptedKey}
	return asn1.Marshal(encryptedPkey)
}

// EncryptPEMBlock takes DER-format bytes and password to return an encrypted PKCS#8 PEM-block
func EncryptPEMBlock(data, password []byte) (*pem.Block, error) {
	encryptedBytes, err := encryptPrivateKey(data, password)
	if err != nil {
		return nil, err
	}

	return &pem.Block{
		Type:    "ENCRYPTED PRIVATE KEY",
		Headers: map[string]string{},
		Bytes:   encryptedBytes,
	}, nil
}

// ConvertECPrivateKeyPEM takes an EC Private Key as input and returns PKCS#8 version of it
func ConvertECPrivateKeyPEM(inPEM []byte) ([]byte, error) {
	block, _ := pem.Decode(inPEM)
	if block == nil {
		return nil, errors.New("invalid pem bytes")
	}

	var ecPrivKey ecPrivateKey
	if _, err := asn1.Unmarshal(block.Bytes, &ecPrivKey); err != nil {
		return nil, errors.New("invalid ec private key")
	}

	var pkey privateKeyInfo
	pkey.Version = 0
	pkey.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 2)
	pkey.PrivateKeyAlgorithm[0] = oidPublicKeyECDSA
	pkey.PrivateKeyAlgorithm[1] = ecPrivKey.NamedCurveOID

	// remove curve oid from private bytes as it is already mentioned in algorithm
	ecPrivKey.NamedCurveOID = nil

	privatekey, err := asn1.Marshal(ecPrivKey)
	if err != nil {
		return nil, err
	}
	pkey.PrivateKey = privatekey

	der, err := asn1.Marshal(pkey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}), nil
}

// ConvertToECPrivateKeyPEM takes an unencrypted PKCS#8 PEM and converts it to
// EC Private Key
func ConvertToECPrivateKeyPEM(inPEM []byte) ([]byte, error) {
	block, _ := pem.Decode(inPEM)
	if block == nil {
		return nil, errors.New("invalid pem bytes")
	}

	var pkey privateKeyInfo
	if _, err := asn1.Unmarshal(block.Bytes, &pkey); err != nil {
		return nil, errors.New("invalid pkcs8 key")
	}

	var ecPrivKey ecPrivateKey
	if _, err := asn1.Unmarshal(pkey.PrivateKey, &ecPrivKey); err != nil {
		return nil, errors.New("invalid private key")
	}

	ecPrivKey.NamedCurveOID = pkey.PrivateKeyAlgorithm[1]
	key, err := asn1.Marshal(ecPrivKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: key,
	}), nil
}
