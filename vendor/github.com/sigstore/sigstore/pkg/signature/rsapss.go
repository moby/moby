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

// checked on LoadSigner, LoadVerifier, and SignMessage
var rsaSupportedHashFuncs = []crypto.Hash{
	crypto.SHA256,
	crypto.SHA384,
	crypto.SHA512,
}

// checked on VerifySignature. Supports SHA1 verification.
var rsaSupportedVerifyHashFuncs = []crypto.Hash{
	crypto.SHA1,
	crypto.SHA256,
	crypto.SHA384,
	crypto.SHA512,
}

// RSAPSSSigner is a signature.Signer that uses the RSA PSS algorithm
type RSAPSSSigner struct {
	hashFunc crypto.Hash
	priv     *rsa.PrivateKey
	pssOpts  *rsa.PSSOptions
}

// LoadRSAPSSSigner calculates signatures using the specified private key and hash algorithm.
//
// If opts are specified, then they will be stored and used as a default if not overridden
// by the value passed to Sign().
//
// hf must be either SHA256, SHA388, or SHA512. opts.Hash is ignored.
func LoadRSAPSSSigner(priv *rsa.PrivateKey, hf crypto.Hash, opts *rsa.PSSOptions) (*RSAPSSSigner, error) {
	if priv == nil {
		return nil, errors.New("invalid RSA private key specified")
	}

	if !isSupportedAlg(hf, rsaSupportedHashFuncs) {
		return nil, errors.New("invalid hash function specified")
	}

	return &RSAPSSSigner{
		priv:     priv,
		pssOpts:  opts,
		hashFunc: hf,
	}, nil
}

// SignMessage signs the provided message using PSS. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the RSAPSSSigner was created.
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
func (r RSAPSSSigner) SignMessage(message io.Reader, opts ...SignOption) ([]byte, error) {
	digest, hf, err := ComputeDigestForSigning(message, r.hashFunc, rsaSupportedHashFuncs, opts...)
	if err != nil {
		return nil, err
	}

	rand := selectRandFromOpts(opts...)
	pssOpts := r.pssOpts
	if pssOpts == nil {
		pssOpts = &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthAuto,
		}
	}
	pssOpts.Hash = hf

	return rsa.SignPSS(rand, r.priv, hf, digest, pssOpts)
}

// Public returns the public key that can be used to verify signatures created by
// this signer.
func (r RSAPSSSigner) Public() crypto.PublicKey {
	if r.priv == nil {
		return nil
	}

	return r.priv.Public()
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (r RSAPSSSigner) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return r.Public(), nil
}

// Sign computes the signature for the specified digest using PSS.
//
// If a source of entropy is given in rand, it will be used instead of the default value (rand.Reader
// from crypto/rand).
//
// If opts are specified, they must be *rsa.PSSOptions. If opts are not specified, the hash function
// provided when the signer was created will be assumed.
func (r RSAPSSSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	rsaOpts := []SignOption{options.WithDigest(digest), options.WithRand(rand)}
	if opts != nil {
		rsaOpts = append(rsaOpts, options.WithCryptoSignerOpts(opts))
	}

	return r.SignMessage(nil, rsaOpts...)
}

// RSAPSSVerifier is a signature.Verifier that uses the RSA PSS algorithm
type RSAPSSVerifier struct {
	publicKey *rsa.PublicKey
	hashFunc  crypto.Hash
	pssOpts   *rsa.PSSOptions
}

// LoadRSAPSSVerifier verifies signatures using the specified public key and hash algorithm.
//
// hf must be either SHA256, SHA388, or SHA512. opts.Hash is ignored.
func LoadRSAPSSVerifier(pub *rsa.PublicKey, hashFunc crypto.Hash, opts *rsa.PSSOptions) (*RSAPSSVerifier, error) {
	if pub == nil {
		return nil, errors.New("invalid RSA public key specified")
	}

	if !isSupportedAlg(hashFunc, rsaSupportedHashFuncs) {
		return nil, errors.New("invalid hash function specified")
	}

	return &RSAPSSVerifier{
		publicKey: pub,
		hashFunc:  hashFunc,
		pssOpts:   opts,
	}, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (r RSAPSSVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return r.publicKey, nil
}

// VerifySignature verifies the signature for the given message using PSS. Unless provided
// in an option, the digest of the message will be computed using the hash function specified
// when the RSAPSSVerifier was created.
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
func (r RSAPSSVerifier) VerifySignature(signature, message io.Reader, opts ...VerifyOption) error {
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

	// rsa.VerifyPSS ignores pssOpts.Hash, so we don't set it
	pssOpts := r.pssOpts
	if pssOpts == nil {
		pssOpts = &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthAuto,
		}
	}

	return rsa.VerifyPSS(r.publicKey, hf, digest, sigBytes, pssOpts)
}

// RSAPSSSignerVerifier is a signature.SignerVerifier that uses the RSA PSS algorithm
type RSAPSSSignerVerifier struct {
	*RSAPSSSigner
	*RSAPSSVerifier
}

// LoadRSAPSSSignerVerifier creates a combined signer and verifier using RSA PSS. This is
// a convenience object that simply wraps an instance of RSAPSSSigner and RSAPSSVerifier.
func LoadRSAPSSSignerVerifier(priv *rsa.PrivateKey, hf crypto.Hash, opts *rsa.PSSOptions) (*RSAPSSSignerVerifier, error) {
	signer, err := LoadRSAPSSSigner(priv, hf, opts)
	if err != nil {
		return nil, fmt.Errorf("initializing signer: %w", err)
	}
	verifier, err := LoadRSAPSSVerifier(&priv.PublicKey, hf, opts)
	if err != nil {
		return nil, fmt.Errorf("initializing verifier: %w", err)
	}

	return &RSAPSSSignerVerifier{
		RSAPSSSigner:   signer,
		RSAPSSVerifier: verifier,
	}, nil
}

// NewDefaultRSAPSSSignerVerifier creates a combined signer and verifier using RSA PSS.
// This creates a new RSA key of 2048 bits and uses the SHA256 hashing algorithm.
func NewDefaultRSAPSSSignerVerifier() (*RSAPSSSignerVerifier, *rsa.PrivateKey, error) {
	return NewRSAPSSSignerVerifier(rand.Reader, 2048, crypto.SHA256)
}

// NewRSAPSSSignerVerifier creates a combined signer and verifier using RSA PSS.
// This creates a new RSA key of the specified length of bits, entropy source, and hash function.
func NewRSAPSSSignerVerifier(rand io.Reader, bits int, hashFunc crypto.Hash) (*RSAPSSSignerVerifier, *rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand, bits)
	if err != nil {
		return nil, nil, err
	}

	sv, err := LoadRSAPSSSignerVerifier(priv, hashFunc, &rsa.PSSOptions{Hash: hashFunc})
	if err != nil {
		return nil, nil, err
	}

	return sv, priv, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (r RSAPSSSignerVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return r.publicKey, nil
}
