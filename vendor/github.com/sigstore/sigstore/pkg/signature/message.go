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
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
)

func isSupportedAlg(alg crypto.Hash, supportedAlgs []crypto.Hash) bool {
	if supportedAlgs == nil {
		return true
	}
	for _, supportedAlg := range supportedAlgs {
		if alg == supportedAlg {
			return true
		}
	}
	return false
}

// ComputeDigestForSigning calculates the digest value for the specified message using a hash function selected by the following process:
//
// - if a digest value is already specified in a SignOption and the length of the digest matches that of the selected hash function, the
// digest value will be returned without any further computation
// - if a hash function is given using WithCryptoSignerOpts(opts) as a SignOption, it will be used (if it is in the supported list)
// - otherwise defaultHashFunc will be used (if it is in the supported list)
func ComputeDigestForSigning(rawMessage io.Reader, defaultHashFunc crypto.Hash, supportedHashFuncs []crypto.Hash, opts ...SignOption) (digest []byte, hashedWith crypto.Hash, err error) {
	var cryptoSignerOpts crypto.SignerOpts = defaultHashFunc
	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&cryptoSignerOpts)
	}
	hashedWith = cryptoSignerOpts.HashFunc()
	if !isSupportedAlg(hashedWith, supportedHashFuncs) {
		return nil, crypto.Hash(0), fmt.Errorf("unsupported hash algorithm: %q not in %v", hashedWith.String(), supportedHashFuncs)
	}
	if len(digest) > 0 {
		if hashedWith != crypto.Hash(0) && len(digest) != hashedWith.Size() {
			err = errors.New("unexpected length of digest for hash function specified")
		}
		return digest, hashedWith, err
	}
	digest, err = hashMessage(rawMessage, hashedWith)
	return digest, hashedWith, err
}

// ComputeDigestForVerifying calculates the digest value for the specified message using a hash function selected by the following process:
//
// - if a digest value is already specified in a SignOption and the length of the digest matches that of the selected hash function, the
// digest value will be returned without any further computation
// - if a hash function is given using WithCryptoSignerOpts(opts) as a SignOption, it will be used (if it is in the supported list)
// - otherwise defaultHashFunc will be used (if it is in the supported list)
func ComputeDigestForVerifying(rawMessage io.Reader, defaultHashFunc crypto.Hash, supportedHashFuncs []crypto.Hash, opts ...VerifyOption) (digest []byte, hashedWith crypto.Hash, err error) {
	var cryptoSignerOpts crypto.SignerOpts = defaultHashFunc
	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&cryptoSignerOpts)
	}
	hashedWith = cryptoSignerOpts.HashFunc()
	if !isSupportedAlg(hashedWith, supportedHashFuncs) {
		return nil, crypto.Hash(0), fmt.Errorf("unsupported hash algorithm: %q not in %v", hashedWith.String(), supportedHashFuncs)
	}
	if len(digest) > 0 {
		if hashedWith != crypto.Hash(0) && len(digest) != hashedWith.Size() {
			err = errors.New("unexpected length of digest for hash function specified")
		}
		return digest, hashedWith, err
	}
	digest, err = hashMessage(rawMessage, hashedWith)
	return digest, hashedWith, err
}

func hashMessage(rawMessage io.Reader, hashFunc crypto.Hash) ([]byte, error) {
	if rawMessage == nil {
		return nil, errors.New("message cannot be nil")
	}
	if hashFunc == crypto.Hash(0) {
		return io.ReadAll(rawMessage)
	}
	hasher := hashFunc.New()
	// avoids reading entire message into memory
	if _, err := io.Copy(hasher, rawMessage); err != nil {
		return nil, fmt.Errorf("hashing message: %w", err)
	}
	return hasher.Sum(nil), nil
}

func selectRandFromOpts(opts ...SignOption) io.Reader {
	rand := crand.Reader
	for _, opt := range opts {
		opt.ApplyRand(&rand)
	}
	return rand
}
