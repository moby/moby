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

package publickey

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

// PublicKey implements verifier.Verifier
type PublicKey struct {
	key crypto.PublicKey
}

func NewVerifier(r io.Reader) (*PublicKey, error) {
	if r == nil {
		return nil, errors.New("public key reader is nil")
	}
	derVerifier, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	key, err := x509.ParsePKIXPublicKey(derVerifier)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %v", err)
	}
	return &PublicKey{key: key}, nil
}

func (k PublicKey) String() string {
	encoded, err := cryptoutils.MarshalPublicKeyToPEM(k.key)
	if err != nil {
		return ""
	}
	return string(encoded)

}

func (k PublicKey) PublicKey() crypto.PublicKey {
	return k.key
}

func (k PublicKey) Identity() (identity.Identity, error) {
	pkixKey, err := cryptoutils.MarshalPublicKeyToDER(k.key)
	if err != nil {
		return identity.Identity{}, err
	}
	digest := sha256.Sum256(pkixKey)
	return identity.Identity{
		Crypto:      k.key,
		Raw:         pkixKey,
		Fingerprint: hex.EncodeToString(digest[:]),
	}, nil
}
