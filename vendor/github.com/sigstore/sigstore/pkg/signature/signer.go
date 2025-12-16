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

	// these ensure we have the implementations loaded
	_ "crypto/sha256"
	_ "crypto/sha512"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// Signer creates digital signatures over a message using a specified key pair
type Signer interface {
	PublicKeyProvider
	SignMessage(message io.Reader, opts ...SignOption) ([]byte, error)
}

// SignerOpts implements crypto.SignerOpts but also allows callers to specify
// additional options that may be utilized in signing the digest provided.
type SignerOpts struct {
	Hash crypto.Hash
	Opts []SignOption
}

// HashFunc returns the hash function for this object
func (s SignerOpts) HashFunc() crypto.Hash {
	return s.Hash
}

// LoadSigner returns a signature.Signer based on the algorithm of the private key
// provided.
//
// If privateKey is an RSA key, a RSAPKCS1v15Signer will be returned. If a
// RSAPSSSigner is desired instead, use the LoadRSAPSSSigner() method directly.
func LoadSigner(privateKey crypto.PrivateKey, hashFunc crypto.Hash) (Signer, error) {
	return LoadSignerWithOpts(privateKey, options.WithHash(hashFunc))
}

// LoadSignerWithOpts returns a signature.Signer based on the algorithm of the private key
// provided.
func LoadSignerWithOpts(privateKey crypto.PrivateKey, opts ...LoadOption) (Signer, error) {
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
			return LoadRSAPSSSigner(pk, hashFunc, rsaPSSOptions)
		}
		return LoadRSAPKCS1v15Signer(pk, hashFunc)
	case *ecdsa.PrivateKey:
		return LoadECDSASigner(pk, hashFunc)
	case ed25519.PrivateKey:
		if useED25519ph {
			return LoadED25519phSigner(pk)
		}
		return LoadED25519Signer(pk)
	}
	return nil, errors.New("unsupported public key type")
}

// LoadSignerFromPEMFile returns a signature.Signer based on the algorithm of the private key
// in the file. The Signer will use the hash function specified when computing digests.
//
// If key is an RSA key, a RSAPKCS1v15Signer will be returned. If a
// RSAPSSSigner is desired instead, use the LoadRSAPSSSigner() and
// cryptoutils.UnmarshalPEMToPrivateKey() methods directly.
func LoadSignerFromPEMFile(path string, hashFunc crypto.Hash, pf cryptoutils.PassFunc) (Signer, error) {
	fileBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	priv, err := cryptoutils.UnmarshalPEMToPrivateKey(fileBytes, pf)
	if err != nil {
		return nil, err
	}
	return LoadSigner(priv, hashFunc)
}

// LoadSignerFromPEMFileWithOpts returns a signature.Signer based on the algorithm of the private key
// in the file. The Signer will use the hash function specified in the options when computing digests.
func LoadSignerFromPEMFileWithOpts(path string, pf cryptoutils.PassFunc, opts ...LoadOption) (Signer, error) {
	fileBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	priv, err := cryptoutils.UnmarshalPEMToPrivateKey(fileBytes, pf)
	if err != nil {
		return nil, err
	}
	return LoadSignerWithOpts(priv, opts...)
}

// LoadDefaultSigner returns a signature.Signer based on the private key.
// Each private key has a corresponding PublicKeyDetails associated in the
// Sigstore ecosystem, see Algorithm Registry for more details.
func LoadDefaultSigner(privateKey crypto.PrivateKey, opts ...LoadOption) (Signer, error) {
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, errors.New("private key does not implement signature.Signer")
	}
	algorithmDetails, err := GetDefaultAlgorithmDetails(signer.Public(), opts...)
	if err != nil {
		return nil, err
	}
	return LoadSignerFromAlgorithmDetails(privateKey, algorithmDetails, opts...)
}

// LoadSignerFromAlgorithmDetails returns a signature.Signer based on
// the algorithm details and the user's choice of options.
func LoadSignerFromAlgorithmDetails(privateKey crypto.PrivateKey, algorithmDetails AlgorithmDetails, opts ...LoadOption) (Signer, error) {
	filteredOpts := GetOptsFromAlgorithmDetails(algorithmDetails, opts...)
	return LoadSignerWithOpts(privateKey, filteredOpts...)
}
