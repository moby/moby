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

// Copied from https://github.com/sigstore/rekor/blob/73dba7c07d0747f00119417fc0ff994a393f97b2/pkg/pki/pki.go

package identity

type Identity struct {
	// Types include:
	// - *rsa.PublicKey
	// - *ecdsa.PublicKey
	// - ed25519.PublicKey
	// - *x509.Certificate
	Crypto any
	// Raw key or certificate extracted from Crypto. Values include:
	// - PKIX ASN.1 DER-encoded public key
	// - ASN.1 DER-encoded certificate
	Raw []byte
	// Contains hex-encoded SHA-256 digest of Raw. Values include:
	// - SHA-256 digest of the PKIX ASN.1 DER-encoded public key
	// - SHA-256 digest of the ASN.1 DER-encoded certificate
	Fingerprint string
}
