// Copyright 2023 The Sigstore Authors.
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

package bundle

import (
	"crypto"
	"crypto/x509"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

type Certificate struct {
	certificate *x509.Certificate
}

func NewCertificate(cert *x509.Certificate) *Certificate {
	return &Certificate{certificate: cert}
}

type PublicKey struct {
	hint string
}

func (pk PublicKey) Hint() string {
	return pk.hint
}

func (c *Certificate) CompareKey(key any, _ root.TrustedMaterial) bool {
	x509Key, ok := key.(*x509.Certificate)
	if !ok {
		return false
	}

	return c.certificate.Equal(x509Key)
}

func (c *Certificate) ValidAtTime(t time.Time, _ root.TrustedMaterial) bool {
	return !c.certificate.NotAfter.Before(t) && !c.certificate.NotBefore.After(t)
}

func (c *Certificate) Certificate() *x509.Certificate {
	return c.certificate
}

func (c *Certificate) PublicKey() verify.PublicKeyProvider {
	return nil
}

func (pk *PublicKey) CompareKey(key any, tm root.TrustedMaterial) bool {
	verifier, err := tm.PublicKeyVerifier(pk.hint)
	if err != nil {
		return false
	}
	pubKey, err := verifier.PublicKey()
	if err != nil {
		return false
	}
	if equaler, ok := key.(interface{ Equal(x crypto.PublicKey) bool }); ok {
		return equaler.Equal(pubKey)
	}
	return false
}

func (pk *PublicKey) ValidAtTime(t time.Time, tm root.TrustedMaterial) bool {
	verifier, err := tm.PublicKeyVerifier(pk.hint)
	if err != nil {
		return false
	}
	return verifier.ValidAtTime(t)
}

func (pk *PublicKey) Certificate() *x509.Certificate {
	return nil
}

func (pk *PublicKey) PublicKey() verify.PublicKeyProvider {
	return pk
}
