//
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

package options

// RequestKeyVersion implements the functional option pattern for specifying the KMS key version during signing or verification
type RequestKeyVersion struct {
	NoOpOptionImpl
	keyVersion string
}

// ApplyKeyVersion sets the KMS's key version as a functional option
func (r RequestKeyVersion) ApplyKeyVersion(keyVersion *string) {
	*keyVersion = r.keyVersion
}

// WithKeyVersion specifies that a specific KMS key version be used during signing and verification operations;
// a value of 0 will use the latest version of the key (default)
func WithKeyVersion(keyVersion string) RequestKeyVersion {
	return RequestKeyVersion{keyVersion: keyVersion}
}

// RequestKeyVersionUsed implements the functional option pattern for obtaining the KMS key version used during signing
type RequestKeyVersionUsed struct {
	NoOpOptionImpl
	keyVersionUsed *string
}

// ApplyKeyVersionUsed requests to store the KMS's key version that was used as a functional option
func (r RequestKeyVersionUsed) ApplyKeyVersionUsed(keyVersionUsed **string) {
	*keyVersionUsed = r.keyVersionUsed
}

// ReturnKeyVersionUsed specifies that the specific KMS key version that was used during signing should be stored
// in the pointer provided
func ReturnKeyVersionUsed(keyVersionUsed *string) RequestKeyVersionUsed {
	return RequestKeyVersionUsed{keyVersionUsed: keyVersionUsed}
}
