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
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha1" // nolint:gosec
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"
)

const (
	// PublicKeyPEMType is the string "PUBLIC KEY" to be used during PEM encoding and decoding
	PublicKeyPEMType PEMType = "PUBLIC KEY"
	// PKCS1PublicKeyPEMType is the string "RSA PUBLIC KEY" used to parse PKCS#1-encoded public keys
	PKCS1PublicKeyPEMType PEMType = "RSA PUBLIC KEY"
)

// subjectPublicKeyInfo is used to construct a subject key ID.
// https://tools.ietf.org/html/rfc5280#section-4.1.2.7
type subjectPublicKeyInfo struct {
	Algorithm        pkix.AlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

// UnmarshalPEMToPublicKey converts a PEM-encoded byte slice into a crypto.PublicKey
func UnmarshalPEMToPublicKey(pemBytes []byte) (crypto.PublicKey, error) {
	derBytes, _ := pem.Decode(pemBytes)
	if derBytes == nil {
		return nil, errors.New("PEM decoding failed")
	}
	switch derBytes.Type {
	case string(PublicKeyPEMType):
		return x509.ParsePKIXPublicKey(derBytes.Bytes)
	case string(PKCS1PublicKeyPEMType):
		return x509.ParsePKCS1PublicKey(derBytes.Bytes)
	default:
		return nil, fmt.Errorf("unknown Public key PEM file type: %v. Are you passing the correct public key?",
			derBytes.Type)
	}
}

// MarshalPublicKeyToDER converts a crypto.PublicKey into a PKIX, ASN.1 DER byte slice
func MarshalPublicKeyToDER(pub crypto.PublicKey) ([]byte, error) {
	if pub == nil {
		return nil, errors.New("empty key")
	}
	return x509.MarshalPKIXPublicKey(pub)
}

// MarshalPublicKeyToPEM converts a crypto.PublicKey into a PEM-encoded byte slice
func MarshalPublicKeyToPEM(pub crypto.PublicKey) ([]byte, error) {
	derBytes, err := MarshalPublicKeyToDER(pub)
	if err != nil {
		return nil, err
	}
	return PEMEncode(PublicKeyPEMType, derBytes), nil
}

// SKID generates a 160-bit SHA-1 hash of the value of the BIT STRING
// subjectPublicKey (excluding the tag, length, and number of unused bits).
// https://tools.ietf.org/html/rfc5280#section-4.2.1.2
func SKID(pub crypto.PublicKey) ([]byte, error) {
	derPubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	var spki subjectPublicKeyInfo
	if _, err := asn1.Unmarshal(derPubBytes, &spki); err != nil {
		return nil, err
	}
	skid := sha1.Sum(spki.SubjectPublicKey.Bytes) // nolint:gosec
	return skid[:], nil
}

// EqualKeys compares two public keys. Supports RSA, ECDSA and ED25519.
// If not equal, the error message contains hex-encoded SHA1 hashes of the DER-encoded keys
func EqualKeys(first, second crypto.PublicKey) error {
	switch pub := first.(type) {
	case *rsa.PublicKey:
		if !pub.Equal(second) {
			return errors.New(genErrMsg(first, second, "rsa"))
		}
	case *ecdsa.PublicKey:
		if !pub.Equal(second) {
			return errors.New(genErrMsg(first, second, "ecdsa"))
		}
	case ed25519.PublicKey:
		if !pub.Equal(second) {
			return errors.New(genErrMsg(first, second, "ed25519"))
		}
	default:
		return errors.New("unsupported key type")
	}
	return nil
}

// genErrMsg generates an error message for EqualKeys
func genErrMsg(first, second crypto.PublicKey, keyType string) string {
	msg := fmt.Sprintf("%s public keys are not equal", keyType)
	// Calculate SKID to include in error message
	firstSKID, err := SKID(first)
	if err != nil {
		return msg
	}
	secondSKID, err := SKID(second)
	if err != nil {
		return msg
	}
	return fmt.Sprintf("%s (%s, %s)", msg, hex.EncodeToString(firstSKID), hex.EncodeToString(secondSKID))
}

// ValidatePubKey validates the parameters of an RSA, ECDSA, or ED25519 public key.
//
// Deprecated: Prefer goodkey.ValidatePubKey. This function has been
// updated to verify only the size of the key for RSA or the curve
// for ECDSA.
func ValidatePubKey(pub crypto.PublicKey) error {
	switch pk := pub.(type) {
	case *rsa.PublicKey:
		if !slices.Contains([]int{2048, 3072, 4096}, pk.Size()*8) {
			return fmt.Errorf("rsa key size %d is not supported supported, modulus size must be 2048, 3072, or 4096", pk.Size()*8)
		}
		return nil
	case *ecdsa.PublicKey:
		if !slices.Contains([]elliptic.Curve{elliptic.P256(), elliptic.P384(), elliptic.P521()}, pk.Curve) {
			return fmt.Errorf("ecdsa curve %T is not supported, must be NIST P-256, P-384 or P-521", pk.Curve)
		}
		return nil
	case ed25519.PublicKey:
		// Nothing to validate for Ed25519
		return nil
	}
	return fmt.Errorf("unsupported public key type: %T", pub)
}
