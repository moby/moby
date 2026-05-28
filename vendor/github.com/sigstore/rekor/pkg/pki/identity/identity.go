// Copyright 2023 The Sigstore Authors.
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

package identity

type Identity struct {
	// Types include:
	// - *rsa.PublicKey
	// - *ecdsa.PublicKey
	// - ed25519.PublicKey
	// - *x509.Certificate
	// - openpgp.EntityList (golang.org/x/crypto/openpgp)
	// - *minisign.PublicKey (github.com/jedisct1/go-minisign)
	// - ssh.PublicKey (golang.org/x/crypto/ssh)
	Crypto any
	// Raw key or certificate extracted from Crypto. Values include:
	// - PKIX ASN.1 DER-encoded public key
	// - ASN.1 DER-encoded certificate
	Raw []byte
	// For keys, certificates, and minisign, hex-encoded SHA-256 digest
	// of the public key or certificate
	// For SSH and PGP, Fingerprint is the standard for each ecosystem
	// For SSH, unpadded base-64 encoded SHA-256 digest of the key
	// For PGP, hex-encoded SHA-1 digest of a key, which can be either
	// a primary key or subkey
	Fingerprint string
}
