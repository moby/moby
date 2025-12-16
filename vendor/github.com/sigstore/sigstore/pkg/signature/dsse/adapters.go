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

// Package dsse includes wrappers to support DSSE
package dsse

import (
	"bytes"
	"context"
	"crypto"
	"errors"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// SignerAdapter wraps a `sigstore/signature.Signer`, making it compatible with `go-securesystemslib/dsse.Signer`.
type SignerAdapter struct {
	SignatureSigner signature.Signer
	Pub             crypto.PublicKey
	Opts            []signature.SignOption
	PubKeyID        string
}

// Sign implements `go-securesystemslib/dsse.Signer`
func (a *SignerAdapter) Sign(ctx context.Context, data []byte) ([]byte, error) {
	return a.SignatureSigner.SignMessage(bytes.NewReader(data), append(a.Opts, options.WithContext(ctx))...)
}

// Verify disabled `go-securesystemslib/dsse.Verifier`
func (a *SignerAdapter) Verify(_ context.Context, _, _ []byte) error {
	return errors.New("Verify disabled")
}

// Public implements `go-securesystemslib/dsse.Verifier`
func (a *SignerAdapter) Public() crypto.PublicKey {
	return a.Pub
}

// KeyID implements `go-securesystemslib/dsse.Verifier`
func (a SignerAdapter) KeyID() (string, error) {
	return a.PubKeyID, nil
}

// VerifierAdapter wraps a `sigstore/signature.Verifier`, making it compatible with `go-securesystemslib/dsse.Verifier`.
type VerifierAdapter struct {
	SignatureVerifier signature.Verifier
	Pub               crypto.PublicKey
	PubKeyID          string
}

// Verify implements `go-securesystemslib/dsse.Verifier`
func (a *VerifierAdapter) Verify(ctx context.Context, data, sig []byte) error {
	return a.SignatureVerifier.VerifySignature(bytes.NewReader(sig), bytes.NewReader(data), options.WithContext(ctx))
}

// Public implements `go-securesystemslib/dsse.Verifier`
func (a *VerifierAdapter) Public() crypto.PublicKey {
	return a.Pub
}

// KeyID implements `go-securesystemslib/dsse.Verifier`
func (a *VerifierAdapter) KeyID() (string, error) {
	return a.PubKeyID, nil
}
