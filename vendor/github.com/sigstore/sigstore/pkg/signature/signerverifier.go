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
	"os"
	"path/filepath"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// SignerVerifier creates and verifies digital signatures over a message using a specified key pair
type SignerVerifier interface {
	Signer
	Verifier
}

// LoadSignerVerifier returns a signature.SignerVerifier based on the algorithm of the private key
// provided.
//
// If privateKey is an RSA key, a RSAPKCS1v15SignerVerifier will be returned. If a
// RSAPSSSignerVerifier is desired instead, use the LoadRSAPSSSignerVerifier() method directly.
func LoadSignerVerifier(privateKey crypto.PrivateKey, hashFunc crypto.Hash) (SignerVerifier, error) {
	return LoadSignerVerifierWithOpts(privateKey, options.WithHash(hashFunc))
}

// LoadSignerVerifierWithOpts returns a signature.SignerVerifier based on the
// algorithm of the private key provided and the user's choice.
func LoadSignerVerifierWithOpts(privateKey crypto.PrivateKey, opts ...LoadOption) (SignerVerifier, error) {
	var rsaPSSOptions *rsa.PSSOptions
	var useED25519ph bool
	hashFunc := crypto.SHA256
	for _, o := range opts {
		o.ApplyED25519ph(&useED25519ph)
		o.ApplyHash(&hashFunc)
		o.ApplyRSAPSS(&rsaPSSOptions)
	}

	switch pk := privateKey.(type) {
	case *rsa.PrivateKey:
		if rsaPSSOptions != nil {
			return LoadRSAPSSSignerVerifier(pk, hashFunc, rsaPSSOptions)
		}
		return LoadRSAPKCS1v15SignerVerifier(pk, hashFunc)
	case *ecdsa.PrivateKey:
		return LoadECDSASignerVerifier(pk, hashFunc)
	case ed25519.PrivateKey:
		if useED25519ph {
			return LoadED25519phSignerVerifier(pk)
		}
		return LoadED25519SignerVerifier(pk)
	}
	return nil, errors.New("unsupported public key type")
}

// LoadSignerVerifierFromPEMFile returns a signature.SignerVerifier based on the algorithm of the private key
// in the file. The SignerVerifier will use the hash function specified when computing digests.
//
// If publicKey is an RSA key, a RSAPKCS1v15SignerVerifier will be returned. If a
// RSAPSSSignerVerifier is desired instead, use the LoadRSAPSSSignerVerifier() and
// cryptoutils.UnmarshalPEMToPrivateKey() methods directly.
func LoadSignerVerifierFromPEMFile(path string, hashFunc crypto.Hash, pf cryptoutils.PassFunc) (SignerVerifier, error) {
	fileBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	priv, err := cryptoutils.UnmarshalPEMToPrivateKey(fileBytes, pf)
	if err != nil {
		return nil, err
	}
	return LoadSignerVerifier(priv, hashFunc)
}

// LoadSignerVerifierFromPEMFileWithOpts returns a signature.SignerVerifier based on the algorithm of the private key
// in the file. The SignerVerifier will use the hash function specified in the options when computing digests.
func LoadSignerVerifierFromPEMFileWithOpts(path string, pf cryptoutils.PassFunc, opts ...LoadOption) (SignerVerifier, error) {
	fileBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	priv, err := cryptoutils.UnmarshalPEMToPrivateKey(fileBytes, pf)
	if err != nil {
		return nil, err
	}
	return LoadSignerVerifierWithOpts(priv, opts...)
}

// LoadDefaultSignerVerifier returns a signature.SignerVerifier based on
// the private key. Each private key has a corresponding PublicKeyDetails
// associated in the Sigstore ecosystem, see Algorithm Registry for more details.
func LoadDefaultSignerVerifier(privateKey crypto.PrivateKey, opts ...LoadOption) (SignerVerifier, error) {
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, errors.New("private key does not implement signature.Signer")
	}
	algorithmDetails, err := GetDefaultAlgorithmDetails(signer.Public(), opts...)
	if err != nil {
		return nil, err
	}
	return LoadSignerVerifierFromAlgorithmDetails(privateKey, algorithmDetails, opts...)
}

// LoadSignerVerifierFromAlgorithmDetails returns a signature.SignerVerifier based on
// the algorithm details and the user's choice of options.
func LoadSignerVerifierFromAlgorithmDetails(privateKey crypto.PrivateKey, algorithmDetails AlgorithmDetails, opts ...LoadOption) (SignerVerifier, error) {
	filteredOpts := GetOptsFromAlgorithmDetails(algorithmDetails, opts...)
	return LoadSignerVerifierWithOpts(privateKey, filteredOpts...)
}
