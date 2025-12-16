// Copyright 2022 The Sigstore Authors.
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

package verification

import (
	"crypto"
	"fmt"

	"github.com/digitorus/timestamp"
	"github.com/pkg/errors"
)

var ErrWeakHashAlg = errors.New("weak hash algorithm: must be SHA-256, SHA-384, or SHA-512")
var ErrUnsupportedHashAlg = errors.New("unsupported hash algorithm")
var ErrInconsistentDigestLength = errors.New("digest length inconsistent with specified hash algorithm")

func VerifyRequest(ts *timestamp.Request) error {
	// only SHA-1, SHA-256, SHA-384, and SHA-512 are supported by the underlying library
	switch ts.HashAlgorithm {
	case crypto.SHA1:
		return ErrWeakHashAlg
	case crypto.SHA256, crypto.SHA384, crypto.SHA512:
	default:
		return ErrUnsupportedHashAlg
	}

	expectedDigestLength := ts.HashAlgorithm.Size()
	actualDigestLength := len(ts.HashedMessage)

	if actualDigestLength != expectedDigestLength {
		return fmt.Errorf("%w: expected %d bytes, got %d bytes", ErrInconsistentDigestLength, expectedDigestLength, actualDigestLength)
	}

	return nil
}
