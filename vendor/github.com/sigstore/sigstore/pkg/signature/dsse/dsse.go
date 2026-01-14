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

package dsse

import (
	"bytes"
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/sigstore/pkg/signature"
)

// WrapSigner returns a signature.Signer that uses the DSSE encoding format
func WrapSigner(s signature.Signer, payloadType string) signature.Signer {
	return &wrappedSigner{
		s:           s,
		payloadType: payloadType,
	}
}

type wrappedSigner struct {
	s           signature.Signer
	payloadType string
}

// PublicKey returns the public key associated with the signer
func (w *wrappedSigner) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return w.s.PublicKey(opts...)
}

// SignMessage signs the provided stream in the reader using the DSSE encoding format
func (w *wrappedSigner) SignMessage(r io.Reader, opts ...signature.SignOption) ([]byte, error) {
	p, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	pae := dsse.PAE(w.payloadType, p)
	sig, err := w.s.SignMessage(bytes.NewReader(pae), opts...)
	if err != nil {
		return nil, err
	}

	env := dsse.Envelope{
		PayloadType: w.payloadType,
		Payload:     base64.StdEncoding.EncodeToString(p),
		Signatures: []dsse.Signature{
			{
				Sig: base64.StdEncoding.EncodeToString(sig),
			},
		},
	}
	return json.Marshal(env)
}

// WrapVerifier returns a signature.Verifier that uses the DSSE encoding format
func WrapVerifier(v signature.Verifier) signature.Verifier {
	return &wrappedVerifier{
		v: v,
	}
}

type wrappedVerifier struct {
	v signature.Verifier
}

// PublicKey returns the public key associated with the verifier
func (w *wrappedVerifier) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return w.v.PublicKey(opts...)
}

// VerifySignature verifies the signature specified in an DSSE envelope
func (w *wrappedVerifier) VerifySignature(s, _ io.Reader, _ ...signature.VerifyOption) error {
	sig, err := io.ReadAll(s)
	if err != nil {
		return err
	}

	env := dsse.Envelope{}
	if err := json.Unmarshal(sig, &env); err != nil {
		return err
	}

	pub, err := w.PublicKey()
	if err != nil {
		return err
	}
	verifier, err := dsse.NewEnvelopeVerifier(&VerifierAdapter{
		SignatureVerifier: w.v,

		Pub:      pub,
		PubKeyID: "", // We do not want to limit verification to a specific key.
	})
	if err != nil {
		return err
	}

	_, err = verifier.Verify(context.Background(), &env)
	return err
}

// WrapSignerVerifier returns a signature.SignerVerifier that uses the DSSE encoding format
func WrapSignerVerifier(sv signature.SignerVerifier, payloadType string) signature.SignerVerifier {
	signer := &wrappedSigner{
		payloadType: payloadType,
		s:           sv,
	}
	verifier := &wrappedVerifier{
		v: sv,
	}

	return &wrappedSignerVerifier{
		signer:   signer,
		verifier: verifier,
	}
}

type wrappedSignerVerifier struct {
	signer   *wrappedSigner
	verifier *wrappedVerifier
}

// PublicKey returns the public key associated with the verifier
func (w *wrappedSignerVerifier) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return w.signer.PublicKey(opts...)
}

// VerifySignature verifies the signature specified in an DSSE envelope
func (w *wrappedSignerVerifier) VerifySignature(s, r io.Reader, opts ...signature.VerifyOption) error {
	return w.verifier.VerifySignature(s, r, opts...)
}

// SignMessage signs the provided stream in the reader using the DSSE encoding format
func (w *wrappedSignerVerifier) SignMessage(r io.Reader, opts ...signature.SignOption) ([]byte, error) {
	return w.signer.SignMessage(r, opts...)
}
