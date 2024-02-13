// Copyright 2015 Google LLC. All Rights Reserved.
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

package ct

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"

	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/x509"
)

// AllowVerificationWithNonCompliantKeys may be set to true in order to allow
// SignatureVerifier to use keys which are technically non-compliant with
// RFC6962.
var AllowVerificationWithNonCompliantKeys = false

// PublicKeyFromPEM parses a PEM formatted block and returns the public key contained within and any remaining unread bytes, or an error.
func PublicKeyFromPEM(b []byte) (crypto.PublicKey, SHA256Hash, []byte, error) {
	p, rest := pem.Decode(b)
	if p == nil {
		return nil, [sha256.Size]byte{}, rest, fmt.Errorf("no PEM block found in %s", string(b))
	}
	k, err := x509.ParsePKIXPublicKey(p.Bytes)
	return k, sha256.Sum256(p.Bytes), rest, err
}

// PublicKeyFromB64 parses a base64-encoded public key.
func PublicKeyFromB64(b64PubKey string) (crypto.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64PubKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding public key: %s", err)
	}
	return x509.ParsePKIXPublicKey(der)
}

// SignatureVerifier can verify signatures on SCTs and STHs
type SignatureVerifier struct {
	PubKey crypto.PublicKey
}

// NewSignatureVerifier creates a new SignatureVerifier using the passed in PublicKey.
func NewSignatureVerifier(pk crypto.PublicKey) (*SignatureVerifier, error) {
	switch pkType := pk.(type) {
	case *rsa.PublicKey:
		if pkType.N.BitLen() < 2048 {
			e := fmt.Errorf("public key is RSA with < 2048 bits (size:%d)", pkType.N.BitLen())
			if !AllowVerificationWithNonCompliantKeys {
				return nil, e
			}
			log.Printf("WARNING: %v", e)
		}
	case *ecdsa.PublicKey:
		params := *(pkType.Params())
		if params != *elliptic.P256().Params() {
			e := fmt.Errorf("public is ECDSA, but not on the P256 curve")
			if !AllowVerificationWithNonCompliantKeys {
				return nil, e
			}
			log.Printf("WARNING: %v", e)

		}
	default:
		return nil, fmt.Errorf("unsupported public key type %v", pkType)
	}

	return &SignatureVerifier{PubKey: pk}, nil
}

// VerifySignature verifies the given signature sig matches the data.
func (s SignatureVerifier) VerifySignature(data []byte, sig tls.DigitallySigned) error {
	return tls.VerifySignature(s.PubKey, data, sig)
}

// VerifySCTSignature verifies that the SCT's signature is valid for the given LogEntry.
func (s SignatureVerifier) VerifySCTSignature(sct SignedCertificateTimestamp, entry LogEntry) error {
	sctData, err := SerializeSCTSignatureInput(sct, entry)
	if err != nil {
		return err
	}
	return s.VerifySignature(sctData, tls.DigitallySigned(sct.Signature))
}

// VerifySTHSignature verifies that the STH's signature is valid.
func (s SignatureVerifier) VerifySTHSignature(sth SignedTreeHead) error {
	sthData, err := SerializeSTHSignatureInput(sth)
	if err != nil {
		return err
	}
	return s.VerifySignature(sthData, tls.DigitallySigned(sth.TreeHeadSignature))
}
