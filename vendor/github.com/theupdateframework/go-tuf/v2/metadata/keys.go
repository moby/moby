// Copyright 2024 The Update Framework Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License
//
// SPDX-License-Identifier: Apache-2.0
//

package metadata

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

const (
	KeyTypeEd25519                = "ed25519"
	KeyTypeECDSA_SHA2_P256_COMPAT = "ecdsa-sha2-nistp256"
	KeyTypeECDSA_SHA2_P256        = "ecdsa"
	KeyTypeRSASSA_PSS_SHA256      = "rsa"
	KeySchemeEd25519              = "ed25519"
	KeySchemeECDSA_SHA2_P256      = "ecdsa-sha2-nistp256"
	KeySchemeECDSA_SHA2_P384      = "ecdsa-sha2-nistp384"
	KeySchemeRSASSA_PSS_SHA256    = "rsassa-pss-sha256"
)

// ToPublicKey generate crypto.PublicKey from metadata type Key
func (k *Key) ToPublicKey() (crypto.PublicKey, error) {
	switch k.Type {
	case KeyTypeRSASSA_PSS_SHA256:
		publicKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(k.Value.PublicKey))
		if err != nil {
			return nil, err
		}
		rsaKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("invalid rsa public key")
		}
		// done for verification - ref. https://github.com/theupdateframework/go-tuf/pull/357
		if _, err := x509.MarshalPKIXPublicKey(rsaKey); err != nil {
			return nil, err
		}
		return rsaKey, nil
	case KeyTypeECDSA_SHA2_P256, KeyTypeECDSA_SHA2_P256_COMPAT: // handle "ecdsa" too as python-tuf/sslib keys are using it for keytype instead of https://theupdateframework.github.io/specification/latest/index.html#keytype-ecdsa-sha2-nistp256
		publicKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(k.Value.PublicKey))
		if err != nil {
			return nil, err
		}
		ecdsaKey, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("invalid ecdsa public key")
		}
		// done for verification - ref. https://github.com/theupdateframework/go-tuf/pull/357
		if _, err := x509.MarshalPKIXPublicKey(ecdsaKey); err != nil {
			return nil, err
		}
		return ecdsaKey, nil
	case KeyTypeEd25519:
		publicKey, err := hex.DecodeString(k.Value.PublicKey)
		if err != nil {
			return nil, err
		}
		ed25519Key := ed25519.PublicKey(publicKey)
		// done for verification - ref. https://github.com/theupdateframework/go-tuf/pull/357
		if _, err := x509.MarshalPKIXPublicKey(ed25519Key); err != nil {
			return nil, err
		}
		return ed25519Key, nil
	}
	return nil, fmt.Errorf("unsupported public key type")
}

// KeyFromPublicKey generate metadata type Key from crypto.PublicKey
func KeyFromPublicKey(k crypto.PublicKey) (*Key, error) {
	key := &Key{}
	switch k := k.(type) {
	case *rsa.PublicKey:
		key.Type = KeyTypeRSASSA_PSS_SHA256
		key.Scheme = KeySchemeRSASSA_PSS_SHA256
		pemKey, err := cryptoutils.MarshalPublicKeyToPEM(k)
		if err != nil {
			return nil, err
		}
		key.Value.PublicKey = string(pemKey)
	case *ecdsa.PublicKey:
		key.Type = KeyTypeECDSA_SHA2_P256
		key.Scheme = KeySchemeECDSA_SHA2_P256
		pemKey, err := cryptoutils.MarshalPublicKeyToPEM(k)
		if err != nil {
			return nil, err
		}
		key.Value.PublicKey = string(pemKey)
	case ed25519.PublicKey:
		key.Type = KeyTypeEd25519
		key.Scheme = KeySchemeEd25519
		key.Value.PublicKey = hex.EncodeToString(k)
	default:
		return nil, fmt.Errorf("unsupported public key type")
	}
	return key, nil
}

// ID returns the keyID value for the given Key, or an error if the key
// cannot be canonically encoded.
func (k *Key) ID() (string, error) {
	// the identifier is a hexdigest of the SHA-256 hash of the canonical form of the key
	if k.id == "" {
		data, err := cjson.EncodeCanonical(k)
		if err != nil {
			return "", fmt.Errorf("error creating key ID: %w", err)
		}
		digest := sha256.Sum256(data)
		k.id = hex.EncodeToString(digest[:])
	}
	return k.id, nil
}
