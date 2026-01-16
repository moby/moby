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
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// Verifier verifies the digital signature using a specified public key
type Verifier interface {
	PublicKeyProvider
	VerifySignature(signature, message io.Reader, opts ...VerifyOption) error
}

// LoadVerifier returns a signature.Verifier based on the algorithm of the public key
// provided that will use the hash function specified when computing digests.
//
// If publicKey is an RSA key, a RSAPKCS1v15Verifier will be returned. If a
// RSAPSSVerifier is desired instead, use the LoadRSAPSSVerifier() method directly.
func LoadVerifier(publicKey crypto.PublicKey, hashFunc crypto.Hash) (Verifier, error) {
	return LoadVerifierWithOpts(publicKey, options.WithHash(hashFunc))
}

// LoadVerifierWithOpts returns a signature.Verifier based on the algorithm of the public key
// provided that will use the hash function specified when computing digests.
func LoadVerifierWithOpts(publicKey crypto.PublicKey, opts ...LoadOption) (Verifier, error) {
	var rsaPSSOptions *rsa.PSSOptions
	var useED25519ph bool
	hashFunc := crypto.SHA256
	for _, o := range opts {
		o.ApplyED25519ph(&useED25519ph)
		o.ApplyHash(&hashFunc)
		o.ApplyRSAPSS(&rsaPSSOptions)
	}

	switch pk := publicKey.(type) {
	case *rsa.PublicKey:
		if rsaPSSOptions != nil {
			return LoadRSAPSSVerifier(pk, hashFunc, rsaPSSOptions)
		}
		return LoadRSAPKCS1v15Verifier(pk, hashFunc)
	case *ecdsa.PublicKey:
		return LoadECDSAVerifier(pk, hashFunc)
	case ed25519.PublicKey:
		if useED25519ph {
			return LoadED25519phVerifier(pk)
		}
		return LoadED25519Verifier(pk)
	}
	return nil, errors.New("unsupported public key type")
}

// LoadUnsafeVerifier returns a signature.Verifier based on the algorithm of the public key
// provided that will use SHA1 when computing digests for RSA and ECDSA signatures.
//
// If publicKey is an RSA key, a RSAPKCS1v15Verifier will be returned. If a
// RSAPSSVerifier is desired instead, use the LoadRSAPSSVerifier() method directly.
func LoadUnsafeVerifier(publicKey crypto.PublicKey) (Verifier, error) {
	switch pk := publicKey.(type) {
	case *rsa.PublicKey:
		if pk == nil {
			return nil, errors.New("invalid RSA public key specified")
		}
		return &RSAPKCS1v15Verifier{
			publicKey: pk,
			hashFunc:  crypto.SHA1,
		}, nil
	case *ecdsa.PublicKey:
		if pk == nil {
			return nil, errors.New("invalid ECDSA public key specified")
		}
		return &ECDSAVerifier{
			publicKey: pk,
			hashFunc:  crypto.SHA1,
		}, nil
	case ed25519.PublicKey:
		return LoadED25519Verifier(pk)
	}
	return nil, errors.New("unsupported public key type")
}

// LoadVerifierFromPEMFile returns a signature.Verifier based on the contents of a
// file located at path. The Verifier wil use the hash function specified when computing digests.
//
// If the publickey is an RSA key, a RSAPKCS1v15Verifier will be returned. If a
// RSAPSSVerifier is desired instead, use the LoadRSAPSSVerifier() and cryptoutils.UnmarshalPEMToPublicKey() methods directly.
func LoadVerifierFromPEMFile(path string, hashFunc crypto.Hash) (Verifier, error) {
	fileBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(fileBytes)
	if err != nil {
		return nil, err
	}

	return LoadVerifier(pubKey, hashFunc)
}

// LoadVerifierFromPEMFileWithOpts returns a signature.Verifier based on the contents of a
// file located at path. The Verifier wil use the hash function specified in the options when computing digests.
func LoadVerifierFromPEMFileWithOpts(path string, opts ...LoadOption) (Verifier, error) {
	fileBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(fileBytes)
	if err != nil {
		return nil, err
	}

	return LoadVerifierWithOpts(pubKey, opts...)
}

// LoadDefaultVerifier returns a signature.Verifier based on the public key.
// Each public key has a corresponding PublicKeyDetails associated in the
// Sigstore ecosystem, see Algorithm Registry for more details.
func LoadDefaultVerifier(publicKey crypto.PublicKey, opts ...LoadOption) (Verifier, error) {
	algorithmDetails, err := GetDefaultAlgorithmDetails(publicKey, opts...)
	if err != nil {
		return nil, err
	}
	return LoadVerifierFromAlgorithmDetails(publicKey, algorithmDetails, opts...)
}

// LoadVerifierFromAlgorithmDetails returns a signature.Verifier based on
// the algorithm details and the user's choice of options.
func LoadVerifierFromAlgorithmDetails(publicKey crypto.PublicKey, algorithmDetails AlgorithmDetails, opts ...LoadOption) (Verifier, error) {
	filteredOpts := GetOptsFromAlgorithmDetails(algorithmDetails, opts...)
	return LoadVerifierWithOpts(publicKey, filteredOpts...)
}
