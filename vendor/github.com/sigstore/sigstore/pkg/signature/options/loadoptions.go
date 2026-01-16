//
// Copyright 2024 The Sigstore Authors.
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

package options

import (
	"crypto"
	"crypto/rsa"
)

// RequestHash implements the functional option pattern for setting a Hash
// function when loading a signer or verifier
type RequestHash struct {
	NoOpOptionImpl
	hashFunc crypto.Hash
}

// ApplyHash sets the hash as requested by the functional option
func (r RequestHash) ApplyHash(hash *crypto.Hash) {
	*hash = r.hashFunc
}

// WithHash specifies that the given hash function should be used when loading a signer or verifier
func WithHash(hash crypto.Hash) RequestHash {
	return RequestHash{hashFunc: hash}
}

// RequestED25519ph implements the functional option pattern for specifying
// ED25519ph (pre-hashed) should be used when loading a signer or verifier and a
// ED25519 key is
type RequestED25519ph struct {
	NoOpOptionImpl
	useED25519ph bool
}

// ApplyED25519ph sets the ED25519ph flag as requested by the functional option
func (r RequestED25519ph) ApplyED25519ph(useED25519ph *bool) {
	*useED25519ph = r.useED25519ph
}

// WithED25519ph specifies that the ED25519ph algorithm should be used when a ED25519 key is used
func WithED25519ph() RequestED25519ph {
	return RequestED25519ph{useED25519ph: true}
}

// RequestPSSOptions implements the functional option pattern for specifying RSA
// PSS should be used when loading a signer or verifier and a RSA key is
// detected
type RequestPSSOptions struct {
	NoOpOptionImpl
	opts *rsa.PSSOptions
}

// ApplyRSAPSS sets the RSAPSS options as requested by the functional option
func (r RequestPSSOptions) ApplyRSAPSS(opts **rsa.PSSOptions) {
	*opts = r.opts
}

// WithRSAPSS specifies that the RSAPSS algorithm should be used when a RSA key is used
// Note that the RSA PSSOptions contains an hash algorithm, which will override
// the hash function specified with WithHash.
func WithRSAPSS(opts *rsa.PSSOptions) RequestPSSOptions {
	return RequestPSSOptions{opts: opts}
}
