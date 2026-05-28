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

package options

// RequestDigest implements the functional option pattern for specifying a digest value
type RequestDigest struct {
	NoOpOptionImpl
	digest []byte
}

// ApplyDigest sets the specified digest value as the functional option
func (r RequestDigest) ApplyDigest(digest *[]byte) {
	*digest = r.digest
}

// WithDigest specifies that the given digest can be used by underlying signature implementations
// WARNING: When verifying a digest with ECDSA, it is trivial to craft a valid signature
// over a random message given a public key. Do not use this unles you understand the
// implications and do not need to protect against malleability.
func WithDigest(digest []byte) RequestDigest {
	return RequestDigest{digest: digest}
}
