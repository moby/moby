// Copyright 2025 The Sigstore Authors
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

package certificate

import (
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/sigstore/rekor-tiles/v2/pkg/verifier/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

// Certificate implements verifier.Verifier
type Certificate struct {
	cert *x509.Certificate
}

func NewVerifier(r io.Reader) (*Certificate, error) {
	if r == nil {
		return nil, errors.New("certificate reader is nil")
	}
	derVerifier, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(derVerifier)
	if err != nil {
		return nil, fmt.Errorf("parsing certificate: %v", err)
	}
	return &Certificate{cert: cert}, nil
}

func (c Certificate) String() string {
	encoded, err := cryptoutils.MarshalCertificateToPEM(c.cert)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func (c Certificate) PublicKey() crypto.PublicKey {
	return c.cert.PublicKey
}

func (c Certificate) Identity() (identity.Identity, error) {
	digest := sha256.Sum256(c.cert.Raw)
	return identity.Identity{
		Crypto:      c.cert,
		Raw:         c.cert.Raw,
		Fingerprint: hex.EncodeToString(digest[:]),
	}, nil
}
