// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

func (p PublicKeyAlgorithm) String() string {
	if p >= total_key_algorithms || p < 0 {
		p = UnknownPublicKeyAlgorithm
	}
	return keyAlgorithmNames[p]
}

func (c *Certificate) SignatureAlgorithmName() string {
	switch c.SignatureAlgorithm {
	case UnknownSignatureAlgorithm:
		return c.SignatureAlgorithmOID.String()
	default:
		return c.SignatureAlgorithm.String()
	}
}

func (c *Certificate) PublicKeyAlgorithmName() string {
	switch c.PublicKeyAlgorithm {
	case UnknownPublicKeyAlgorithm:
		return c.PublicKeyAlgorithmOID.String()
	default:
		return c.PublicKeyAlgorithm.String()
	}
}
