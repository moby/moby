//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cryptoutils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/secure-systems-lab/go-securesystemslib/encrypted"
)

const (
	// PrivateKeyPEMType is the string "PRIVATE KEY" to be used during PEM encoding and decoding
	PrivateKeyPEMType PEMType = "PRIVATE KEY"
	// ECPrivateKeyPEMType is the string "EC PRIVATE KEY" used to parse SEC 1 EC private keys
	ECPrivateKeyPEMType PEMType = "EC PRIVATE KEY"
	// PKCS1PrivateKeyPEMType is the string "RSA PRIVATE KEY" used to parse PKCS#1-encoded private keys
	PKCS1PrivateKeyPEMType           PEMType = "RSA PRIVATE KEY"
	encryptedCosignPrivateKeyPEMType PEMType = "ENCRYPTED COSIGN PRIVATE KEY"
	// EncryptedSigstorePrivateKeyPEMType is the string "ENCRYPTED SIGSTORE PRIVATE KEY" to be used during PEM encoding and decoding
	EncryptedSigstorePrivateKeyPEMType PEMType = "ENCRYPTED SIGSTORE PRIVATE KEY"
)

func pemEncodeKeyPair(priv crypto.PrivateKey, pub crypto.PublicKey, pf PassFunc) (privPEM, pubPEM []byte, err error) {
	pubPEM, err = MarshalPublicKeyToPEM(pub)
	if err != nil {
		return nil, nil, err
	}
	derBytes, err := MarshalPrivateKeyToDER(priv)
	if err != nil {
		return nil, nil, err
	}

	if pf == nil {
		return PEMEncode(PrivateKeyPEMType, derBytes), pubPEM, nil
	}
	password, err := pf(true)
	if err != nil {
		return nil, nil, err
	}
	if password == nil {
		return PEMEncode(PrivateKeyPEMType, derBytes), pubPEM, nil
	}
	if derBytes, err = encrypted.Encrypt(derBytes, password); err != nil {
		return nil, nil, err
	}
	return PEMEncode(EncryptedSigstorePrivateKeyPEMType, derBytes), pubPEM, nil
}

// GeneratePEMEncodedECDSAKeyPair generates an ECDSA keypair, optionally password encrypted using a provided PassFunc, and PEM encoded.
func GeneratePEMEncodedECDSAKeyPair(curve elliptic.Curve, pf PassFunc) (privPEM, pubPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return pemEncodeKeyPair(priv, priv.Public(), pf)
}

// GeneratePEMEncodedRSAKeyPair generates an RSA keypair, optionally password encrypted using a provided PassFunc, and PEM encoded.
func GeneratePEMEncodedRSAKeyPair(keyLengthBits int, pf PassFunc) (privPEM, pubPEM []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, keyLengthBits)
	if err != nil {
		return nil, nil, err
	}
	return pemEncodeKeyPair(priv, priv.Public(), pf)
}

// MarshalPrivateKeyToEncryptedDER marshals the private key and encrypts the DER-encoded value using the specified password function
func MarshalPrivateKeyToEncryptedDER(priv crypto.PrivateKey, pf PassFunc) ([]byte, error) {
	derKey, err := MarshalPrivateKeyToDER(priv)
	if err != nil {
		return nil, err
	}
	password, err := pf(true)
	if err != nil {
		return nil, err
	}
	if password == nil {
		return nil, errors.New("password was nil")
	}
	return encrypted.Encrypt(derKey, password)
}

// UnmarshalPEMToPrivateKey converts a PEM-encoded byte slice into a crypto.PrivateKey
func UnmarshalPEMToPrivateKey(pemBytes []byte, pf PassFunc) (crypto.PrivateKey, error) {
	derBlock, _ := pem.Decode(pemBytes)
	if derBlock == nil {
		return nil, errors.New("PEM decoding failed")
	}
	switch derBlock.Type {
	case string(PrivateKeyPEMType):
		return x509.ParsePKCS8PrivateKey(derBlock.Bytes)
	case string(PKCS1PrivateKeyPEMType):
		return x509.ParsePKCS1PrivateKey(derBlock.Bytes)
	case string(ECPrivateKeyPEMType):
		return x509.ParseECPrivateKey(derBlock.Bytes)
	case string(EncryptedSigstorePrivateKeyPEMType), string(encryptedCosignPrivateKeyPEMType):
		derBytes := derBlock.Bytes
		if pf != nil {
			password, err := pf(false)
			if err != nil {
				return nil, err
			}
			if password != nil {
				derBytes, err = encrypted.Decrypt(derBytes, password)
				if err != nil {
					return nil, err
				}
			}
		}

		return x509.ParsePKCS8PrivateKey(derBytes)
	}
	return nil, fmt.Errorf("unknown private key PEM file type: %v", derBlock.Type)
}

// MarshalPrivateKeyToDER converts a crypto.PrivateKey into a PKCS8 ASN.1 DER byte slice
func MarshalPrivateKeyToDER(priv crypto.PrivateKey) ([]byte, error) {
	if priv == nil {
		return nil, errors.New("empty key")
	}
	return x509.MarshalPKCS8PrivateKey(priv)
}

// MarshalPrivateKeyToPEM converts a crypto.PrivateKey into a PKCS#8 PEM-encoded byte slice
func MarshalPrivateKeyToPEM(priv crypto.PrivateKey) ([]byte, error) {
	derBytes, err := MarshalPrivateKeyToDER(priv)
	if err != nil {
		return nil, err
	}
	return PEMEncode(PrivateKeyPEMType, derBytes), nil
}
