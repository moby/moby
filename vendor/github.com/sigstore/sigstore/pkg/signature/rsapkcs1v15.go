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
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"

	"github.com/sigstore/sigstore/pkg/signature/options"
)

// RSAPKCS1v15Signer is a signature.Signer that uses the RSA PKCS1v15 algorithm
type RSAPKCS1v15Signer struct {
	hashFunc crypto.Hash
	priv     *rsa.PrivateKey
}

// LoadRSAPKCS1v15Signer calculates signatures using the specified private key and hash algorithm.
//
// hf must be either SHA256, SHA388, or SHA512.
func LoadRSAPKCS1v15Signer(priv *rsa.PrivateKey, hf crypto.Hash) (*RSAPKCS1v15Signer, error) {
	if priv == nil {
		return nil, errors.New("invalid RSA private key specified")
	}

	if !isSupportedAlg(hf, rsaSupportedHashFuncs) {
		return nil, errors.New("invalid hash function specified")
	}

	return &RSAPKCS1v15Signer{
		priv:     priv,
		hashFunc: hf,
	}, nil
}

// SignMessage signs the provided message using PKCS1v15. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the RSAPKCS1v15Signer was created.
//
// SignMessage recognizes the following Options listed in order of preference:
//
// - WithRand()
//
// - WithDigest()
//
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (r RSAPKCS1v15Signer) SignMessage(message io.Reader, opts ...SignOption) ([]byte, error) {
	digest, hf, err := ComputeDigestForSigning(message, r.hashFunc, rsaSupportedHashFuncs, opts...)
	if err != nil {
		return nil, err
	}

	rand := selectRandFromOpts(opts...)

	return rsa.SignPKCS1v15(rand, r.priv, hf, digest)
}

// Public returns the public key that can be used to verify signatures created by
// this signer.
func (r RSAPKCS1v15Signer) Public() crypto.PublicKey {
	if r.priv == nil {
		return nil
	}

	return r.priv.Public()
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (r RSAPKCS1v15Signer) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return r.Public(), nil
}

// Sign computes the signature for the specified digest using PKCS1v15.
//
// If a source of entropy is given in rand, it will be used instead of the default value (rand.Reader
// from crypto/rand).
//
// If opts are specified, they should specify the hash function used to compute digest. If opts are
// not specified, this function assumes the hash function provided when the signer was created was
// used to create the value specified in digest.
func (r RSAPKCS1v15Signer) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	rsaOpts := []SignOption{options.WithDigest(digest), options.WithRand(rand)}
	if opts != nil {
		rsaOpts = append(rsaOpts, options.WithCryptoSignerOpts(opts))
	}

	return r.SignMessage(nil, rsaOpts...)
}

// RSAPKCS1v15Verifier is a signature.Verifier that uses the RSA PKCS1v15 algorithm
type RSAPKCS1v15Verifier struct {
	publicKey *rsa.PublicKey
	hashFunc  crypto.Hash
}

// LoadRSAPKCS1v15Verifier returns a Verifier that verifies signatures using the specified
// RSA public key and hash algorithm.
//
// hf must be either SHA256, SHA388, or SHA512.
func LoadRSAPKCS1v15Verifier(pub *rsa.PublicKey, hashFunc crypto.Hash) (*RSAPKCS1v15Verifier, error) {
	if pub == nil {
		return nil, errors.New("invalid RSA public key specified")
	}

	if !isSupportedAlg(hashFunc, rsaSupportedHashFuncs) {
		return nil, errors.New("invalid hash function specified")
	}

	return &RSAPKCS1v15Verifier{
		publicKey: pub,
		hashFunc:  hashFunc,
	}, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (r RSAPKCS1v15Verifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return r.publicKey, nil
}

// VerifySignature verifies the signature for the given message using PKCS1v15. Unless provided
// in an option, the digest of the message will be computed using the hash function specified
// when the RSAPKCS1v15Verifier was created.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithDigest()
//
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (r RSAPKCS1v15Verifier) VerifySignature(signature, message io.Reader, opts ...VerifyOption) error {
	digest, hf, err := ComputeDigestForVerifying(message, r.hashFunc, rsaSupportedVerifyHashFuncs, opts...)
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

	return rsa.VerifyPKCS1v15(r.publicKey, hf, digest, sigBytes)
}

// RSAPKCS1v15SignerVerifier is a signature.SignerVerifier that uses the RSA PKCS1v15 algorithm
type RSAPKCS1v15SignerVerifier struct {
	*RSAPKCS1v15Signer
	*RSAPKCS1v15Verifier
}

// LoadRSAPKCS1v15SignerVerifier creates a combined signer and verifier. This is a convenience object
// that simply wraps an instance of RSAPKCS1v15Signer and RSAPKCS1v15Verifier.
func LoadRSAPKCS1v15SignerVerifier(priv *rsa.PrivateKey, hf crypto.Hash) (*RSAPKCS1v15SignerVerifier, error) {
	signer, err := LoadRSAPKCS1v15Signer(priv, hf)
	if err != nil {
		return nil, fmt.Errorf("initializing signer: %w", err)
	}
	verifier, err := LoadRSAPKCS1v15Verifier(&priv.PublicKey, hf)
	if err != nil {
		return nil, fmt.Errorf("initializing verifier: %w", err)
	}

	return &RSAPKCS1v15SignerVerifier{
		RSAPKCS1v15Signer:   signer,
		RSAPKCS1v15Verifier: verifier,
	}, nil
}

// NewDefaultRSAPKCS1v15SignerVerifier creates a combined signer and verifier using RSA PKCS1v15.
// This creates a new RSA key of 2048 bits and uses the SHA256 hashing algorithm.
func NewDefaultRSAPKCS1v15SignerVerifier() (*RSAPKCS1v15SignerVerifier, *rsa.PrivateKey, error) {
	return NewRSAPKCS1v15SignerVerifier(rand.Reader, 2048, crypto.SHA256)
}

// NewRSAPKCS1v15SignerVerifier creates a combined signer and verifier using RSA PKCS1v15.
// This creates a new RSA key of the specified length of bits, entropy source, and hash function.
func NewRSAPKCS1v15SignerVerifier(rand io.Reader, bits int, hashFunc crypto.Hash) (*RSAPKCS1v15SignerVerifier, *rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand, bits)
	if err != nil {
		return nil, nil, err
	}

	sv, err := LoadRSAPKCS1v15SignerVerifier(priv, hashFunc)
	if err != nil {
		return nil, nil, err
	}

	return sv, priv, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (r RSAPKCS1v15SignerVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return r.publicKey, nil
}
