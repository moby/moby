package mozilla

/*
 * ZLint Copyright 2023 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type ecdsaSignatureAidEncoding struct{}

/************************************************
https://www.mozilla.org/en-US/about/governance/policies/security-group/certs/policy/

When a root or intermediate certificate's ECDSA key is used to produce a signature, only the following algorithms may
be used, and with the following encoding requirements:

If the signing key is P-256, the signature MUST use ECDSA with SHA-256. The encoded AlgorithmIdentifier MUST match the
following hex-encoded bytes: 300a06082a8648ce3d040302.

If the signing key is P-384, the signature MUST use ECDSA with SHA-384. The encoded AlgorithmIdentifier MUST match the
following hex-encoded bytes: 300a06082a8648ce3d040303.

The above encodings consist of the corresponding OID with the parameters field omitted, as specified by RFC 5758,
Section 3.2. Certificates MUST NOT include a NULL parameter. Note this differs from RSASSA-PKCS1-v1_5, which includes
an explicit NULL.

************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_mp_ecdsa_signature_encoding_correct",
		Description:   "The encoded algorithm identifiers for ECDSA signatures MUST match specific hex-encoded bytes",
		Citation:      "Mozilla Root Store Policy / Section 5.1.2",
		Source:        lint.MozillaRootStorePolicy,
		EffectiveDate: util.MozillaPolicy27Date,
		Lint:          NewEcdsaSignatureAidEncoding,
	})
}

func NewEcdsaSignatureAidEncoding() lint.LintInterface {
	return &ecdsaSignatureAidEncoding{}
}

func (l *ecdsaSignatureAidEncoding) CheckApplies(c *x509.Certificate) bool {
	// check for all ECDSA signature algorithms to avoid missing this lint if an unsupported algorithm is used in the first place
	// 1.2.840.10045.4.3.1 is SHA224withECDSA
	return c.SignatureAlgorithm == x509.ECDSAWithSHA1 ||
		c.SignatureAlgorithm == x509.ECDSAWithSHA256 ||
		c.SignatureAlgorithm == x509.ECDSAWithSHA384 ||
		c.SignatureAlgorithm == x509.ECDSAWithSHA512 ||
		c.SignatureAlgorithmOID.Equal(util.OidSignatureSHA224withECDSA)
}

func (l *ecdsaSignatureAidEncoding) Execute(c *x509.Certificate) *lint.LintResult {
	// We must check consistency of the issuer public key to the signature algorithm
	// (see for example: If the signing key is P-256, the signature MUST use ECDSA with SHA-256.
	// The encoded AlgorithmIdentifier MUST match the following hex-encoded bytes: 300a06082a8648ce3d040302.)
	// Thus we need the issuer public key which it is not available so easy.
	// At this stage all certificates (also of sub-CAs and root-CAs, provided they are linted) are either
	// P-256 or P-384 (see lint e_mp_ecdsa_pub_key_encoding_correct).
	// Therefore we check the length of the signature in the certificate. If it is 0 ... 72 bytes then it is
	// assumed done by a P-256 key and if it is 73 ... 104 bytes it is assumed done by a P-384 key.

	signature := c.Signature
	signatureSize := len(signature)
	encoded, err := util.GetSignatureAlgorithmInTBSEncoded(c)
	if err != nil {
		return &lint.LintResult{Status: lint.Error, Details: err.Error()}
	}

	// Signatures made with P-256 are not going to be greater than 72 bytes long
	// Seq Tag+Length = 2, r Tag+length = 2, s Tag+length = 2, r max 32+1 (unsigned representation), same for s
	// len <= 2+2+2+33+33 (= 72)
	const maxP256SigByteLen = 72
	// len <= 2+2+2+49+49 (= 104)
	const maxP384SigByteLen = 104

	if signatureSize <= maxP256SigByteLen {
		expectedEncoding := []byte{0x30, 0x0a, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x04, 0x03, 0x02}

		if bytes.Equal(encoded, expectedEncoding) {
			return &lint.LintResult{Status: lint.Pass}
		}
		return &lint.LintResult{
			Status:  lint.Error,
			Details: fmt.Sprintf("Encoding of signature algorithm does not match signing key on P-256 curve. Got the unsupported %s", hex.EncodeToString(encoded)),
		}
	} else if signatureSize <= maxP384SigByteLen {
		expectedEncoding := []byte{0x30, 0x0a, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x04, 0x03, 0x03}

		if bytes.Equal(encoded, expectedEncoding) {
			return &lint.LintResult{Status: lint.Pass}
		}
		return &lint.LintResult{
			Status:  lint.Error,
			Details: fmt.Sprintf("Encoding of signature algorithm does not match signing key on P-384 curve. Got the unsupported %s", hex.EncodeToString(encoded)),
		}
	}
	return &lint.LintResult{
		Status:  lint.Error,
		Details: fmt.Sprintf("Encoding of signature algorithm does not match signing key. Got signature length %v", signatureSize),
	}
}
