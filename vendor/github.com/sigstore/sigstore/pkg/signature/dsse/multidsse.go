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
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/sigstore/pkg/signature"
)

type wrappedMultiSigner struct {
	sLAdapters  []dsse.Signer
	payloadType string
}

// WrapMultiSigner returns a signature.Signer that uses the DSSE encoding format
func WrapMultiSigner(payloadType string, sL ...signature.Signer) signature.Signer {
	signerAdapterL := make([]dsse.Signer, 0, len(sL))
	for _, s := range sL {
		pub, err := s.PublicKey()
		if err != nil {
			return nil
		}

		keyID, err := dsse.SHA256KeyID(pub)
		if err != nil {
			keyID = ""
		}

		signerAdapter := &SignerAdapter{
			SignatureSigner: s,
			Pub:             s.PublicKey,
			PubKeyID:        keyID, // We do not want to limit verification to a specific key.
		}

		signerAdapterL = append(signerAdapterL, signerAdapter)
	}

	return &wrappedMultiSigner{
		sLAdapters:  signerAdapterL,
		payloadType: payloadType,
	}
}

// PublicKey returns the public key associated with the signer
func (wL *wrappedMultiSigner) PublicKey(_ ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return nil, errors.New("not supported for multi signatures")
}

// SignMessage signs the provided stream in the reader using the DSSE encoding format
func (wL *wrappedMultiSigner) SignMessage(r io.Reader, _ ...signature.SignOption) ([]byte, error) {
	p, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	envSigner, err := dsse.NewEnvelopeSigner(wL.sLAdapters...)
	if err != nil {
		return nil, err
	}

	env, err := envSigner.SignPayload(context.Background(), wL.payloadType, p)
	if err != nil {
		return nil, err
	}

	return json.Marshal(env)
}

type wrappedMultiVerifier struct {
	vLAdapters  []dsse.Verifier
	threshold   int
	payloadType string
}

// WrapMultiVerifier returns a signature.Verifier that uses the DSSE encoding format
func WrapMultiVerifier(payloadType string, threshold int, vL ...signature.Verifier) signature.Verifier {
	verifierAdapterL := make([]dsse.Verifier, 0, len(vL))
	for _, v := range vL {
		pub, err := v.PublicKey()
		if err != nil {
			return nil
		}

		keyID, err := dsse.SHA256KeyID(pub)
		if err != nil {
			keyID = ""
		}

		verifierAdapter := &VerifierAdapter{
			SignatureVerifier: v,
			Pub:               v.PublicKey,
			PubKeyID:          keyID, // We do not want to limit verification to a specific key.
		}

		verifierAdapterL = append(verifierAdapterL, verifierAdapter)
	}

	return &wrappedMultiVerifier{
		vLAdapters:  verifierAdapterL,
		payloadType: payloadType,
		threshold:   threshold,
	}
}

// PublicKey returns the public key associated with the signer
func (wL *wrappedMultiVerifier) PublicKey(_ ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return nil, errors.New("not supported for multi signatures")
}

// VerifySignature verifies the signature specified in an DSSE envelope
func (wL *wrappedMultiVerifier) VerifySignature(s, _ io.Reader, _ ...signature.VerifyOption) error {
	sig, err := io.ReadAll(s)
	if err != nil {
		return err
	}

	env := dsse.Envelope{}
	if err := json.Unmarshal(sig, &env); err != nil {
		return err
	}

	envVerifier, err := dsse.NewMultiEnvelopeVerifier(wL.threshold, wL.vLAdapters...)
	if err != nil {
		return err
	}

	_, err = envVerifier.Verify(context.Background(), &env)
	return err
}

// WrapMultiSignerVerifier returns a signature.SignerVerifier that uses the DSSE encoding format
func WrapMultiSignerVerifier(payloadType string, threshold int, svL ...signature.SignerVerifier) signature.SignerVerifier {
	signerL := make([]signature.Signer, 0, len(svL))
	verifierL := make([]signature.Verifier, 0, len(svL))
	for _, sv := range svL {
		signerL = append(signerL, sv)
		verifierL = append(verifierL, sv)
	}

	sL := WrapMultiSigner(payloadType, signerL...)
	vL := WrapMultiVerifier(payloadType, threshold, verifierL...)

	return &wrappedMultiSignerVerifier{
		signer:   sL,
		verifier: vL,
	}
}

type wrappedMultiSignerVerifier struct {
	signer   signature.Signer
	verifier signature.Verifier
}

// PublicKey returns the public key associated with the verifier
func (w *wrappedMultiSignerVerifier) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return w.signer.PublicKey(opts...)
}

// VerifySignature verifies the signature specified in an DSSE envelope
func (w *wrappedMultiSignerVerifier) VerifySignature(s, r io.Reader, opts ...signature.VerifyOption) error {
	return w.verifier.VerifySignature(s, r, opts...)
}

// SignMessage signs the provided stream in the reader using the DSSE encoding format
func (w *wrappedMultiSignerVerifier) SignMessage(r io.Reader, opts ...signature.SignOption) ([]byte, error) {
	return w.signer.SignMessage(r, opts...)
}
