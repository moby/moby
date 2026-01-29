//
// Copyright 2024 The Sigstore Authors.
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
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/sigstore/sigstore/pkg/signature/options"
)

var ed25519phSupportedHashFuncs = []crypto.Hash{
	crypto.SHA512,
}

// ED25519phSigner is a signature.Signer that uses the Ed25519 public-key signature system with pre-hashing
type ED25519phSigner struct {
	priv ed25519.PrivateKey
}

// LoadED25519phSigner calculates signatures using the specified private key.
func LoadED25519phSigner(priv ed25519.PrivateKey) (*ED25519phSigner, error) {
	if priv == nil {
		return nil, errors.New("invalid ED25519 private key specified")
	}

	return &ED25519phSigner{
		priv: priv,
	}, nil
}

// ToED25519SignerVerifier creates a ED25519SignerVerifier from a ED25519phSignerVerifier
//
// Clients that use ED25519phSignerVerifier should use this method to get a
// SignerVerifier that uses the same ED25519 private key, but with the Pure
// Ed25519 algorithm. This might be necessary to interact with Fulcio, which
// only supports the Pure Ed25519 algorithm.
func (e ED25519phSignerVerifier) ToED25519SignerVerifier() (*ED25519SignerVerifier, error) {
	return LoadED25519SignerVerifier(e.priv)
}

// SignMessage signs the provided message. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the ED25519phSigner was created.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithDigest()
//
// All other options are ignored if specified.
func (e ED25519phSigner) SignMessage(message io.Reader, opts ...SignOption) ([]byte, error) {
	digest, _, err := ComputeDigestForSigning(message, crypto.SHA512, ed25519phSupportedHashFuncs, opts...)
	if err != nil {
		return nil, err
	}

	return e.priv.Sign(nil, digest, crypto.SHA512)
}

// Public returns the public key that can be used to verify signatures created by
// this signer.
func (e ED25519phSigner) Public() crypto.PublicKey {
	if e.priv == nil {
		return nil
	}

	return e.priv.Public()
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e ED25519phSigner) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.Public(), nil
}

// Sign computes the signature for the specified message; the first and third arguments to this
// function are ignored as they are not used by the ED25519ph algorithm.
func (e ED25519phSigner) Sign(_ io.Reader, digest []byte, _ crypto.SignerOpts) ([]byte, error) {
	return e.SignMessage(nil, options.WithDigest(digest))
}

// ED25519phVerifier is a signature.Verifier that uses the Ed25519 public-key signature system
type ED25519phVerifier struct {
	publicKey ed25519.PublicKey
}

// LoadED25519phVerifier returns a Verifier that verifies signatures using the
// specified ED25519 public key.
func LoadED25519phVerifier(pub ed25519.PublicKey) (*ED25519phVerifier, error) {
	if pub == nil {
		return nil, errors.New("invalid ED25519 public key specified")
	}

	return &ED25519phVerifier{
		publicKey: pub,
	}, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e *ED25519phVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.publicKey, nil
}

// VerifySignature verifies the signature for the given message. Unless provided
// in an option, the digest of the message will be computed using the hash function specified
// when the ED25519phVerifier was created.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithDigest()
//
// All other options are ignored if specified.
func (e *ED25519phVerifier) VerifySignature(signature, message io.Reader, opts ...VerifyOption) error {
	if signature == nil {
		return errors.New("nil signature passed to VerifySignature")
	}

	digest, _, err := ComputeDigestForVerifying(message, crypto.SHA512, ed25519phSupportedHashFuncs, opts...)
	if err != nil {
		return err
	}

	sigBytes, err := io.ReadAll(signature)
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}

	if err := ed25519.VerifyWithOptions(e.publicKey, digest, sigBytes, &ed25519.Options{Hash: crypto.SHA512}); err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}
	return nil
}

// ED25519phSignerVerifier is a signature.SignerVerifier that uses the Ed25519 public-key signature system
type ED25519phSignerVerifier struct {
	*ED25519phSigner
	*ED25519phVerifier
}

// LoadED25519phSignerVerifier creates a combined signer and verifier. This is
// a convenience object that simply wraps an instance of ED25519phSigner and ED25519phVerifier.
func LoadED25519phSignerVerifier(priv ed25519.PrivateKey) (*ED25519phSignerVerifier, error) {
	signer, err := LoadED25519phSigner(priv)
	if err != nil {
		return nil, fmt.Errorf("initializing signer: %w", err)
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("given key is not ed25519.PublicKey")
	}
	verifier, err := LoadED25519phVerifier(pub)
	if err != nil {
		return nil, fmt.Errorf("initializing verifier: %w", err)
	}

	return &ED25519phSignerVerifier{
		ED25519phSigner:   signer,
		ED25519phVerifier: verifier,
	}, nil
}

// NewDefaultED25519phSignerVerifier creates a combined signer and verifier using ED25519.
// This creates a new ED25519 key using crypto/rand as an entropy source.
func NewDefaultED25519phSignerVerifier() (*ED25519phSignerVerifier, ed25519.PrivateKey, error) {
	return NewED25519phSignerVerifier(rand.Reader)
}

// NewED25519phSignerVerifier creates a combined signer and verifier using ED25519.
// This creates a new ED25519 key using the specified entropy source.
func NewED25519phSignerVerifier(rand io.Reader) (*ED25519phSignerVerifier, ed25519.PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(rand)
	if err != nil {
		return nil, nil, err
	}

	sv, err := LoadED25519phSignerVerifier(priv)
	if err != nil {
		return nil, nil, err
	}

	return sv, priv, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e ED25519phSignerVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.publicKey, nil
}
