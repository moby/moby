package cabf_br

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

type algorithmObjectIdentifierEncoding struct{}

/*
***********************************************
This lint refers to CAB Baseline Requirements (Version 1.7.4) chapter 7.1.3.1, which defines the
required encodings of AlgorithmObjectIdentifiers inside a SubjectPublicKeyInfo field.

Section 7.1.3.1.1: When encoded, the AlgorithmIdentifier for RSA keys MUST be byte‐for‐byte
identical with the following hex‐encoded bytes: 300d06092a864886f70d0101010500

Section 7.1.3.1.2: When encoded, the AlgorithmIdentifier for ECDSA keys MUST be
byte‐for‐byte identical with the following hex‐encoded bytes:
For P‐256 keys: 301306072a8648ce3d020106082a8648ce3d030107
For P‐384 keys: 301006072a8648ce3d020106052b81040022
For P‐521 keys: 301006072a8648ce3d020106052b81040023
***********************************************
*/
func init() {
	lint.RegisterLint(&lint.Lint{
		Name: "e_algorithm_identifier_improper_encoding",
		Description: "Encoded AlgorithmObjectIdentifier objects inside a SubjectPublicKeyInfo field " +
			"MUST comply with specified byte sequences.",
		Citation:      "BRs: 7.1.3.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABFBRs_1_7_1_Date,
		Lint:          NewAlgorithmObjectIdentifierEncoding,
	})
}

func NewAlgorithmObjectIdentifierEncoding() lint.LintInterface {
	return &algorithmObjectIdentifierEncoding{}
}

var allowedPublicKeyEncodings = [4][]byte{
	// encoded AlgorithmIdentifier for an RSA key
	{0x30, 0x0d, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01, 0x01, 0x05, 0x00},
	// encoded AlgorithmIdentifier for a P-256 key
	{0x30, 0x13, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07},
	// encoded AlgorithmIdentifier for a P-384 key
	{0x30, 0x10, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, 0x06, 0x05, 0x2b, 0x81, 0x04, 0x00, 0x22},
	// encoded AlgorithmIdentifier for a P-521 key
	{0x30, 0x10, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, 0x06, 0x05, 0x2b, 0x81, 0x04, 0x00, 0x23},
}

func (l *algorithmObjectIdentifierEncoding) CheckApplies(c *x509.Certificate) bool {
	// always check if the public key is one of the four explicitly specified encodings
	return true
}

func (l *algorithmObjectIdentifierEncoding) Execute(c *x509.Certificate) *lint.LintResult {

	rawAlgorithmIdentifier, err := util.GetPublicKeyAidEncoded(c)
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal, Details: "error parsing SubjectPublicKeyInfo"}
	}

	for _, encoding := range allowedPublicKeyEncodings {
		if bytes.Equal(rawAlgorithmIdentifier, encoding) {
			return &lint.LintResult{Status: lint.Pass}
		}
	}

	return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("The encoded AlgorithmObjectIdentifier %q inside the SubjectPublicKeyInfo field is not allowed", hex.EncodeToString(rawAlgorithmIdentifier))}
}
