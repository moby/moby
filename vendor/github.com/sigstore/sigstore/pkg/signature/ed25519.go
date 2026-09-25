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
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

var ed25519SupportedHashFuncs = []crypto.Hash{
	crypto.Hash(0),
}

// ED25519Signer is a signature.Signer that uses the Ed25519 public-key signature system
type ED25519Signer struct {
	priv ed25519.PrivateKey
}

// LoadED25519Signer calculates signatures using the specified private key.
func LoadED25519Signer(priv ed25519.PrivateKey) (*ED25519Signer, error) {
	if priv == nil {
		return nil, errors.New("invalid ED25519 private key specified")
	}

	// check this to avoid panic and throw error gracefully
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid size for ED25519 key")
	}

	return &ED25519Signer{
		priv: priv,
	}, nil
}

// SignMessage signs the provided message. Passing the WithDigest option is not
// supported as ED25519 performs a two pass hash over the message during the
// signing process.
//
// All options are ignored.
func (e ED25519Signer) SignMessage(message io.Reader, _ ...SignOption) ([]byte, error) {
	messageBytes, _, err := ComputeDigestForSigning(message, crypto.Hash(0), ed25519SupportedHashFuncs)
	if err != nil {
		return nil, err
	}

	return ed25519.Sign(e.priv, messageBytes), nil
}

// Public returns the public key that can be used to verify signatures created by
// this signer.
func (e ED25519Signer) Public() crypto.PublicKey {
	if e.priv == nil {
		return nil
	}

	return e.priv.Public()
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e ED25519Signer) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.Public(), nil
}

// Sign computes the signature for the specified message; the first and third arguments to this
// function are ignored as they are not used by the ED25519 algorithm.
func (e ED25519Signer) Sign(_ io.Reader, message []byte, _ crypto.SignerOpts) ([]byte, error) {
	if message == nil {
		return nil, errors.New("message must not be nil")
	}
	return e.SignMessage(bytes.NewReader(message))
}

// ED25519Verifier is a signature.Verifier that uses the Ed25519 public-key signature system
type ED25519Verifier struct {
	publicKey ed25519.PublicKey
}

// LoadED25519Verifier returns a Verifier that verifies signatures using the specified ED25519 public key.
func LoadED25519Verifier(pub ed25519.PublicKey) (*ED25519Verifier, error) {
	if pub == nil {
		return nil, errors.New("invalid ED25519 public key specified")
	}

	return &ED25519Verifier{
		publicKey: pub,
	}, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e *ED25519Verifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.publicKey, nil
}

// VerifySignature verifies the signature for the given message.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// All options are ignored if specified.
func (e *ED25519Verifier) VerifySignature(signature, message io.Reader, _ ...VerifyOption) error {
	messageBytes, _, err := ComputeDigestForVerifying(message, crypto.Hash(0), ed25519SupportedHashFuncs)
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

	if !ed25519.Verify(e.publicKey, messageBytes, sigBytes) {
		return errors.New("failed to verify signature")
	}
	return nil
}

// ED25519SignerVerifier is a signature.SignerVerifier that uses the Ed25519 public-key signature system
type ED25519SignerVerifier struct {
	*ED25519Signer
	*ED25519Verifier
}

// LoadED25519SignerVerifier creates a combined signer and verifier. This is
// a convenience object that simply wraps an instance of ED25519Signer and ED25519Verifier.
func LoadED25519SignerVerifier(priv ed25519.PrivateKey) (*ED25519SignerVerifier, error) {
	signer, err := LoadED25519Signer(priv)
	if err != nil {
		return nil, fmt.Errorf("initializing signer: %w", err)
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("given key is not ed25519.PublicKey")
	}
	verifier, err := LoadED25519Verifier(pub)
	if err != nil {
		return nil, fmt.Errorf("initializing verifier: %w", err)
	}

	return &ED25519SignerVerifier{
		ED25519Signer:   signer,
		ED25519Verifier: verifier,
	}, nil
}

// NewDefaultED25519SignerVerifier creates a combined signer and verifier using ED25519.
// This creates a new ED25519 key using crypto/rand as an entropy source.
func NewDefaultED25519SignerVerifier() (*ED25519SignerVerifier, ed25519.PrivateKey, error) {
	return NewED25519SignerVerifier(rand.Reader)
}

// NewED25519SignerVerifier creates a combined signer and verifier using ED25519.
// This creates a new ED25519 key using the specified entropy source.
func NewED25519SignerVerifier(rand io.Reader) (*ED25519SignerVerifier, ed25519.PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(rand)
	if err != nil {
		return nil, nil, err
	}

	sv, err := LoadED25519SignerVerifier(priv)
	if err != nil {
		return nil, nil, err
	}

	return sv, priv, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (e ED25519SignerVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return e.publicKey, nil
}
