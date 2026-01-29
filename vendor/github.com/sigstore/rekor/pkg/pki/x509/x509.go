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

package x509

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/sigstore/rekor/pkg/pki/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	sigsig "github.com/sigstore/sigstore/pkg/signature"
)

// EmailAddressOID defined by https://oidref.com/1.2.840.113549.1.9.1
var EmailAddressOID asn1.ObjectIdentifier = []int{1, 2, 840, 113549, 1, 9, 1}

type Signature struct {
	signature        []byte
	verifierLoadOpts []sigsig.LoadOption
}

// NewSignature creates and validates an x509 signature object
func NewSignature(r io.Reader) (*Signature, error) {
	return NewSignatureWithOpts(r)
}

func NewSignatureWithOpts(r io.Reader, opts ...sigsig.LoadOption) (*Signature, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &Signature{
		signature:        b,
		verifierLoadOpts: opts,
	}, nil
}

// CanonicalValue implements the pki.Signature interface
func (s Signature) CanonicalValue() ([]byte, error) {
	return s.signature, nil
}

// Verify implements the pki.Signature interface
func (s Signature) Verify(r io.Reader, k interface{}, opts ...sigsig.VerifyOption) error {
	if len(s.signature) == 0 {
		//lint:ignore ST1005 X509 is proper use of term
		return errors.New("X509 signature has not been initialized")
	}

	key, ok := k.(*PublicKey)
	if !ok {
		return fmt.Errorf("invalid public key type for: %v", k)
	}

	p := key.key
	if p == nil {
		switch {
		case key.cert != nil:
			p = key.cert.c.PublicKey
		case len(key.certs) > 0:
			if err := verifyCertChain(key.certs); err != nil {
				return err
			}
			p = key.certs[0].PublicKey
		default:
			return errors.New("no public key found")
		}
	}

	verifier, err := sigsig.LoadVerifierWithOpts(p, s.verifierLoadOpts...)
	if err != nil {
		return err
	}
	return verifier.VerifySignature(bytes.NewReader(s.signature), r, opts...)
}

// PublicKey Public Key that follows the x509 standard
type PublicKey struct {
	key   interface{}
	cert  *cert
	certs []*x509.Certificate
}

type cert struct {
	c *x509.Certificate
	b []byte
}

// NewPublicKey implements the pki.PublicKey interface
func NewPublicKey(r io.Reader) (*PublicKey, error) {
	rawPub, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	trimmedRawPub := bytes.TrimSpace(rawPub)

	block, rest := pem.Decode(trimmedRawPub)
	if block == nil {
		return nil, errors.New("invalid public key: failure decoding PEM")
	}

	// Handle certificate chain, concatenated PEM-encoded certificates
	if len(rest) > 0 {
		// Support up to 10 certificates in a chain, to avoid parsing extremely long chains
		certs, err := cryptoutils.UnmarshalCertificatesFromPEMLimited(trimmedRawPub, 10)
		if err != nil {
			return nil, err
		}
		return &PublicKey{certs: certs}, nil
	}

	switch block.Type {
	case string(cryptoutils.PublicKeyPEMType):
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return &PublicKey{key: key}, nil
	case string(cryptoutils.CertificatePEMType):
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		return &PublicKey{
			cert: &cert{
				c: c,
				b: block.Bytes,
			}}, nil
	}
	return nil, fmt.Errorf("invalid public key: cannot handle type %v", block.Type)
}

// CanonicalValue implements the pki.PublicKey interface
func (k PublicKey) CanonicalValue() (encoded []byte, err error) {

	switch {
	case k.key != nil:
		encoded, err = cryptoutils.MarshalPublicKeyToPEM(k.key)
	case k.cert != nil:
		encoded, err = cryptoutils.MarshalCertificateToPEM(k.cert.c)
	case k.certs != nil:
		encoded, err = cryptoutils.MarshalCertificatesToPEM(k.certs)
	default:
		err = errors.New("x509 public key has not been initialized")
	}

	return
}

func (k PublicKey) CryptoPubKey() crypto.PublicKey {
	if k.cert != nil {
		return k.cert.c.PublicKey
	}
	if len(k.certs) > 0 {
		return k.certs[0].PublicKey
	}
	return k.key
}

// EmailAddresses implements the pki.PublicKey interface
func (k PublicKey) EmailAddresses() []string {
	var names []string
	var cert *x509.Certificate
	if k.cert != nil {
		cert = k.cert.c
	} else if len(k.certs) > 0 {
		cert = k.certs[0]
	}
	if cert != nil {
		for _, name := range cert.EmailAddresses {
			if govalidator.IsEmail(name) {
				names = append(names, strings.ToLower(name))
			}
		}
	}
	return names
}

// Subjects implements the pki.PublicKey interface
func (k PublicKey) Subjects() []string {
	var subjects []string
	var cert *x509.Certificate
	if k.cert != nil {
		cert = k.cert.c
	} else if len(k.certs) > 0 {
		cert = k.certs[0]
	}
	if cert != nil {
		subjects = cryptoutils.GetSubjectAlternateNames(cert)
	}
	return subjects
}

// Identities implements the pki.PublicKey interface
func (k PublicKey) Identities() ([]identity.Identity, error) {
	// k contains either a key, a cert, or a list of certs
	if k.key != nil {
		pkixKey, err := cryptoutils.MarshalPublicKeyToDER(k.key)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(pkixKey)
		return []identity.Identity{{
			Crypto:      k.key,
			Raw:         pkixKey,
			Fingerprint: hex.EncodeToString(digest[:]),
		}}, nil
	}

	var cert *x509.Certificate
	switch {
	case k.cert != nil:
		cert = k.cert.c
	case len(k.certs) > 0:
		cert = k.certs[0]
	default:
		return nil, errors.New("no key, certificate or certificate chain provided")
	}

	digest := sha256.Sum256(cert.Raw)
	return []identity.Identity{{
		Crypto:      cert,
		Raw:         cert.Raw,
		Fingerprint: hex.EncodeToString(digest[:]),
	}}, nil
}

func verifyCertChain(certChain []*x509.Certificate) error {
	if len(certChain) == 0 {
		return errors.New("no certificate chain provided")
	}
	// No certificate chain to verify
	if len(certChain) == 1 {
		return nil
	}
	rootPool := x509.NewCertPool()
	rootPool.AddCert(certChain[len(certChain)-1])
	subPool := x509.NewCertPool()
	for _, c := range certChain[1 : len(certChain)-1] {
		subPool.AddCert(c)
	}
	if _, err := certChain[0].Verify(x509.VerifyOptions{
		Roots:         rootPool,
		Intermediates: subPool,
		// Allow any key usage
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		// Expired certificates can be uploaded and should be verifiable
		CurrentTime: certChain[0].NotBefore,
	}); err != nil {
		return err
	}
	return nil
}
