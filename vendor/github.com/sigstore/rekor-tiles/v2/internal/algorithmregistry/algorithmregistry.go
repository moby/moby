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

package algorithmregistry

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"
	"reflect"

	v1 "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/sigstore/sigstore/pkg/signature"
)

var (
	// AllowedClientSigningAlgorithms is the default set of supported signing
	// algorithms for log entry signatures.
	AllowedClientSigningAlgorithms = []v1.PublicKeyDetails{
		v1.PublicKeyDetails_PKIX_RSA_PKCS1V15_2048_SHA256,
		v1.PublicKeyDetails_PKIX_RSA_PKCS1V15_3072_SHA256,
		v1.PublicKeyDetails_PKIX_RSA_PKCS1V15_4096_SHA256,
		v1.PublicKeyDetails_PKIX_ECDSA_P256_SHA_256,
		v1.PublicKeyDetails_PKIX_ECDSA_P384_SHA_384,
		v1.PublicKeyDetails_PKIX_ECDSA_P521_SHA_512,
		v1.PublicKeyDetails_PKIX_ED25519,
		v1.PublicKeyDetails_PKIX_ED25519_PH,
	}
)

type UnsupportedAlgorithm struct {
	Pub crypto.PublicKey
	Alg crypto.Hash
}

func (e *UnsupportedAlgorithm) Error() string {
	hash := e.Alg.String()

	switch v := e.Pub.(type) {
	case *rsa.PublicKey:
		bits := v.Size() * 8
		return fmt.Sprintf("unsupported entry algorithm for RSA key, size %d, digest %s", bits, hash)
	case *ecdsa.PublicKey:
		name := v.Curve.Params().Name
		return fmt.Sprintf("unsupported entry algorithm for ECDSA key, curve %s, digest %s", name, hash)
	case ed25519.PublicKey:
		return fmt.Sprintf("unsupported entry algorithm for Ed25519 key, digest %s", hash)
	default:
		return fmt.Sprintf("unsupported key type %s, digest %s", reflect.TypeOf(v), hash)
	}
}

// AlgorithmRegistry accepts a list of algorithms as strings, parses and formats them into a registry.
func AlgorithmRegistry(algorithmOptions []string) (*signature.AlgorithmRegistryConfig, error) {
	var algorithms []v1.PublicKeyDetails
	if algorithmOptions == nil {
		algorithms = AllowedClientSigningAlgorithms
	} else {
		for _, a := range algorithmOptions {
			algorithm, err := signature.ParseSignatureAlgorithmFlag(a)
			if err != nil {
				return nil, fmt.Errorf("parsing signature algorithm flag: %w", err)
			}
			algorithms = append(algorithms, algorithm)
		}
	}
	algorithmsStr := make([]string, len(algorithms))
	var err error
	for i, a := range algorithms {
		algorithmsStr[i], err = signature.FormatSignatureAlgorithmFlag(a)
		if err != nil {
			return nil, fmt.Errorf("formatting signature algorithm flag: %w", err)
		}
	}
	algorithmRegistry, err := signature.NewAlgorithmRegistryConfig(algorithms)
	if err != nil {
		return nil, fmt.Errorf("getting algorithm registry: %w", err)
	}
	return algorithmRegistry, nil
}

// CheckEntryAlgorithms checks that the combination public key and message
// digest algorithm are allowed given an algorithm registry.
func CheckEntryAlgorithms(pubKey crypto.PublicKey, alg crypto.Hash, algorithmRegistry *signature.AlgorithmRegistryConfig) (bool, error) {
	// Check if all the verifiers public keys (together with the
	// artifactHashValue) are allowed according to the policy
	isPermitted, err := algorithmRegistry.IsAlgorithmPermitted(pubKey, alg)
	if err != nil {
		return false, fmt.Errorf("checking if algorithm is permitted: %w", err)
	}
	if !isPermitted {
		return false, nil
	}
	return true, nil
}
