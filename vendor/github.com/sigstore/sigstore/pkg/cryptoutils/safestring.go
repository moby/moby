//
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

package cryptoutils

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateRandomURLSafeString generates a cryptographically secure random
// URL-safe string with the specified number of bits of entropy.
func GenerateRandomURLSafeString(entropyLength uint) string {
	if entropyLength == 0 {
		return ""
	}
	// Round up to the nearest byte to ensure minimum entropy is met
	entropyBytes := (entropyLength + 7) / 8
	b := make([]byte, entropyBytes)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
