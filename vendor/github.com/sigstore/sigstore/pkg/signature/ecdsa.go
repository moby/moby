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

package signature

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/sigstore/sigstore/pkg/signature/options"
)

// checked on LoadSigner, LoadVerifier and SignMessage
var ecdsaSupportedHashFuncs = []crypto.Hash{
	crypto.SHA256,
	crypto.SHA512,
	crypto.SHA384,
	crypto.SHA224,
}

// checked on VerifySignature. Supports SHA1 verification.
var ecdsaSupportedVerifyHashFuncs = []crypto.Hash{
	crypto.SHA256,
	crypto.SHA512,
	crypto.SHA384,
	crypto.SHA224,
	crypto.SHA1,
}

// ECDSASigner is a signature.Signer that uses an Elliptic Curve DSA algorithm
type ECDSASigner struct {
	hashFunc crypto.Hash
	priv     *ecdsa.PrivateKey
}

// LoadECDSASigner calculates signatures using the specified private key and hash algorithm.
//
// hf must not be crypto.Hash(0).
func LoadECDSASigner(priv *ecdsa.PrivateKey, hf crypto.Hash) (*ECDSASigner, error) {
	if priv == nil {
		return nil, errors.New("invalid ECDSA private key specified")
	}

	if !isSupportedAlg(hf, ecdsaSupportedHashFuncs) {
		return nil, errors.New("invalid hash function specified")
	}

	return &ECDSASigner{
		priv:     priv,
		hashFunc: hf,
	}, nil
}

// SignMessage signs the provided message. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the ECDSASigner was created.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithRand()
//
// - WithDigest()
//
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (e ECDSASigner) SignMessage(message io.Reader, opts ...SignOption) ([]byte, error) {
	digest, _, err := ComputeDigestForSigning(message, e.hashFunc, ecdsaSupportedHashFuncs, opts...)
	if err != nil {
		return nil, err
	}

	rand := selectRandFromOpts(opts...)

	return ecdsa.SignASN1(rand, e.priv, digest)
}

// Public returns the public key that can be used to verify signatures created by
// this signer.
func (e ECDSASigner) Public() crypto.PublicKey {
	if e.priv == nil {
		return nil
	}

	return e.priv.Public()
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e ECDSASigner) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.Public(), nil
}

// Sign computes the signature for the specified digest. If a source of entropy is
// given in rand, it will be used instead of the default value (rand.Reader from crypto/rand).
//
// If opts are specified, the hash function in opts.Hash should be the one used to compute
// digest. If opts are not specified, the value provided when the signer was created will be used instead.
func (e ECDSASigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	ecdsaOpts := []SignOption{options.WithDigest(digest), options.WithRand(rand)}
	if opts != nil {
		ecdsaOpts = append(ecdsaOpts, options.WithCryptoSignerOpts(opts))
	}

	return e.SignMessage(nil, ecdsaOpts...)
}

// ECDSAVerifier is a signature.Verifier that uses an Elliptic Curve DSA algorithm
type ECDSAVerifier struct {
	publicKey *ecdsa.PublicKey
	hashFunc  crypto.Hash
}

// LoadECDSAVerifier returns a Verifier that verifies signatures using the specified
// ECDSA public key and hash algorithm.
//
// hf must not be crypto.Hash(0).
func LoadECDSAVerifier(pub *ecdsa.PublicKey, hashFunc crypto.Hash) (*ECDSAVerifier, error) {
	if pub == nil {
		return nil, errors.New("invalid ECDSA public key specified")
	}

	if !isSupportedAlg(hashFunc, ecdsaSupportedHashFuncs) {
		return nil, errors.New("invalid hash function specified")
	}

	return &ECDSAVerifier{
		publicKey: pub,
		hashFunc:  hashFunc,
	}, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e ECDSAVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.publicKey, nil
}

// VerifySignature verifies the signature for the given message. Unless provided
// in an option, the digest of the message will be computed using the hash function specified
// when the ECDSAVerifier was created.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithDigest()
//
// All other options are ignored if specified.
func (e ECDSAVerifier) VerifySignature(signature, message io.Reader, opts ...VerifyOption) error {
	if e.publicKey == nil {
		return errors.New("no public key set for ECDSAVerifier")
	}

	digest, _, err := ComputeDigestForVerifying(message, e.hashFunc, ecdsaSupportedVerifyHashFuncs, opts...)
	if err != nil {
		return err
	}

	if signature == nil {
		return errors.New("nil signature passed to VerifySignature")
	}

	sigBytes, err := io.ReadAll(signature)
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}

	// Without this check, VerifyASN1 panics on an invalid key.
	if !e.publicKey.IsOnCurve(e.publicKey.X, e.publicKey.Y) {
		return fmt.Errorf("invalid ECDSA public key for %s", e.publicKey.Params().Name)
	}

	asnParseTest := struct {
		R, S *big.Int
	}{}
	if _, err := asn1.Unmarshal(sigBytes, &asnParseTest); err == nil {
		if !ecdsa.VerifyASN1(e.publicKey, digest, sigBytes) {
			return errors.New("invalid signature when validating ASN.1 encoded signature")
		}
	} else {
		// deal with IEEE P1363 encoding of signatures
		if len(sigBytes) == 0 || len(sigBytes) > 132 || len(sigBytes)%2 != 0 {
			return errors.New("ecdsa: Invalid IEEE_P1363 encoded bytes")
		}
		r := new(big.Int).SetBytes(sigBytes[:len(sigBytes)/2])
		s := new(big.Int).SetBytes(sigBytes[len(sigBytes)/2:])
		if !ecdsa.Verify(e.publicKey, digest, r, s) {
			return errors.New("invalid signature when validating IEEE_P1363 encoded signature")
		}
	}

	return nil
}

// ECDSASignerVerifier is a signature.SignerVerifier that uses an Elliptic Curve DSA algorithm
type ECDSASignerVerifier struct {
	*ECDSASigner
	*ECDSAVerifier
}

// LoadECDSASignerVerifier creates a combined signer and verifier. This is a convenience object
// that simply wraps an instance of ECDSASigner and ECDSAVerifier.
func LoadECDSASignerVerifier(priv *ecdsa.PrivateKey, hf crypto.Hash) (*ECDSASignerVerifier, error) {
	signer, err := LoadECDSASigner(priv, hf)
	if err != nil {
		return nil, fmt.Errorf("initializing signer: %w", err)
	}
	verifier, err := LoadECDSAVerifier(&priv.PublicKey, hf)
	if err != nil {
		return nil, fmt.Errorf("initializing verifier: %w", err)
	}

	return &ECDSASignerVerifier{
		ECDSASigner:   signer,
		ECDSAVerifier: verifier,
	}, nil
}

// NewDefaultECDSASignerVerifier creates a combined signer and verifier using ECDSA.
//
// This creates a new ECDSA key using the P-256 curve and uses the SHA256 hashing algorithm.
func NewDefaultECDSASignerVerifier() (*ECDSASignerVerifier, *ecdsa.PrivateKey, error) {
	return NewECDSASignerVerifier(elliptic.P256(), rand.Reader, crypto.SHA256)
}

// NewECDSASignerVerifier creates a combined signer and verifier using ECDSA.
//
// This creates a new ECDSA key using the specified elliptic curve, entropy source, and hashing function.
func NewECDSASignerVerifier(curve elliptic.Curve, rand io.Reader, hashFunc crypto.Hash) (*ECDSASignerVerifier, *ecdsa.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(curve, rand)
	if err != nil {
		return nil, nil, err
	}

	sv, err := LoadECDSASignerVerifier(priv, hashFunc)
	if err != nil {
		return nil, nil, err
	}

	return sv, priv, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e ECDSASignerVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.publicKey, nil
}
