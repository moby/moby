// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
)

// CertificateFingerprint represents a digest/fingerprint of some data. It can
// easily be encoded to hex and JSON (as a hex string).
type CertificateFingerprint []byte

// MD5Fingerprint creates a fingerprint of data using the MD5 hash algorithm.
func MD5Fingerprint(data []byte) CertificateFingerprint {
	sum := md5.Sum(data)
	return sum[:]
}

// SHA1Fingerprint creates a fingerprint of data using the SHA1 hash algorithm.
func SHA1Fingerprint(data []byte) CertificateFingerprint {
	sum := sha1.Sum(data)
	return sum[:]
}

// SHA256Fingerprint creates a fingerprint of data using the SHA256 hash
// algorithm.
func SHA256Fingerprint(data []byte) CertificateFingerprint {
	sum := sha256.Sum256(data)
	return sum[:]
}

// SHA512Fingerprint creates a fingerprint of data using the SHA256 hash
// algorithm.
func SHA512Fingerprint(data []byte) CertificateFingerprint {
	sum := sha512.Sum512(data)
	return sum[:]
}

// Equal returns true if the fingerprints are bytewise-equal.
func (f CertificateFingerprint) Equal(other CertificateFingerprint) bool {
	return bytes.Equal(f, other)
}

// Hex returns the given fingerprint encoded as a hex string.
func (f CertificateFingerprint) Hex() string {
	return hex.EncodeToString(f)
}

// MarshalJSON implements the json.Marshaler interface, and marshals the
// fingerprint as a hex string.
func (f *CertificateFingerprint) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Hex())
}
