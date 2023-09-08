package mozilla

/*
 * ZLint Copyright 2021 Regents of the University of Michigan
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

type rsaPssAidEncoding struct{}

/************************************************

https://www.mozilla.org/en-US/about/governance/policies/security-group/certs/policy/

Section 5.1.1 RSA

RSASSA-PSS with SHA-256, MGF-1 with SHA-256, and a salt length of 32 bytes.

The encoded AlgorithmIdentifier MUST match the following hex-encoded bytes:

304106092a864886f70d01010a3034a00f300d0609608648016503040201
0500a11c301a06092a864886f70d010108300d0609608648016503040201
0500a203020120

RSASSA-PSS with SHA-384, MGF-1 with SHA-384, and a salt length of 48 bytes.

The encoded AlgorithmIdentifier MUST match the following hex-encoded bytes:

304106092a864886f70d01010a3034a00f300d0609608648016503040202
0500a11c301a06092a864886f70d010108300d0609608648016503040202
0500a203020130

RSASSA-PSS with SHA-512, MGF-1 with SHA-512, and a salt length of 64 bytes.

The encoded AlgorithmIdentifier MUST match the following hex-encoded bytes:

304106092a864886f70d01010a3034a00f300d0609608648016503040203
0500a11c301a06092a864886f70d010108300d0609608648016503040203
0500a203020140
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_mp_rsassa-pss_parameters_encoding_in_signature_algorithm_correct",
		Description:   "The encoded AlgorithmIdentifier for RSASSA-PSS in the signature algorithm MUST match specific bytes",
		Citation:      "Mozilla Root Store Policy / Section 5.1.1",
		Source:        lint.MozillaRootStorePolicy,
		EffectiveDate: util.MozillaPolicy27Date,
		Lint:          &rsaPssAidEncoding{},
	})
}

var RSASSAPSSAlgorithmIDToDER = [3][]byte{
	// RSASSA-PSS with SHA-256, MGF-1 with SHA-256, salt length 32 bytes
	{0x30, 0x41, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01, 0x0a, 0x30, 0x34, 0xa0, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0xa1, 0x1c, 0x30, 0x1a, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01, 0x08, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0xa2, 0x03, 0x02, 0x01, 0x20},
	// RSASSA-PSS with SHA-384, MGF-1 with SHA-384, salt length 48 bytes
	{0x30, 0x41, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01, 0x0a, 0x30, 0x34, 0xa0, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x02, 0x05, 0x00, 0xa1, 0x1c, 0x30, 0x1a, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01, 0x08, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x02, 0x05, 0x00, 0xa2, 0x03, 0x02, 0x01, 0x30},
	// RSASSA-PSS with SHA-512, MGF-1 with SHA-512, salt length 64 bytes
	{0x30, 0x41, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01, 0x0a, 0x30, 0x34, 0xa0, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x03, 0x05, 0x00, 0xa1, 0x1c, 0x30, 0x1a, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01, 0x08, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x03, 0x05, 0x00, 0xa2, 0x03, 0x02, 0x01, 0x40},
}

func (l *rsaPssAidEncoding) Initialize() error {
	return nil
}

func (l *rsaPssAidEncoding) CheckApplies(c *x509.Certificate) bool {
	return c.SignatureAlgorithmOID.Equal(util.OidRSASSAPSS)
}

func (l *rsaPssAidEncoding) Execute(c *x509.Certificate) *lint.LintResult {
	signatureAlgoID, err := util.GetSignatureAlgorithmInTBSEncoded(c)
	if err != nil {
		return &lint.LintResult{Status: lint.Error, Details: err.Error()}
	}

	for _, encoding := range RSASSAPSSAlgorithmIDToDER {
		if bytes.Equal(signatureAlgoID, encoding) {
			return &lint.LintResult{Status: lint.Pass}
		}
	}

	return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("RSASSA-PSS parameters are not properly encoded. %v presentations are allowed but got the unsupported %s", len(RSASSAPSSAlgorithmIDToDER), hex.EncodeToString(signatureAlgoID))}
}
