// Copyright 2025 The Sigstore Authors.
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

package verifier

import (
	"crypto"

	"github.com/sigstore/rekor-tiles/v2/pkg/verifier/identity"
)

// Verifier represents a structure that can verify a signature, e.g. a public key or certificate
type Verifier interface {
	// PublicKey returns the underlying public key for signature verification
	PublicKey() crypto.PublicKey
	// Identity returns the identity of the verifier from a key or certificate
	Identity() (identity.Identity, error)
	// String returns a human-readable representation of the verifier, e.g. PEM-encoded
	String() string
}
