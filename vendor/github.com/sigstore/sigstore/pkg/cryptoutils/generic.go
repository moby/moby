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

package cryptoutils

import (
	"encoding/pem"
)

// PEMType is a specific type for string constants used during PEM encoding and decoding
type PEMType string

// PEMEncode encodes the specified byte slice in PEM format using the provided type string
func PEMEncode(typeStr PEMType, bytes []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  string(typeStr),
		Bytes: bytes,
	})
}
