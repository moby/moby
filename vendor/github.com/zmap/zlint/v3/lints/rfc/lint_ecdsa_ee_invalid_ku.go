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

package rfc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type ecdsaInvalidKU struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "n_ecdsa_ee_invalid_ku",
		Description:   "ECDSA end-entity certificates MAY have key usages: digitalSignature, nonRepudiation and keyAgreement",
		Citation:      "RFC 5480 Section 3",
		Source:        lint.RFC5480,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewEcdsaInvalidKU,
	})
}

func NewEcdsaInvalidKU() lint.LintInterface {
	return &ecdsaInvalidKU{}
}

// Initialize is a no-op for this lint.

// CheckApplies returns true when the certificate is a subscriber cert using an
// ECDSA public key algorithm.
func (l *ecdsaInvalidKU) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && c.PublicKeyAlgorithm == x509.ECDSA
}

// Execute returns a Notice level lint.LintResult if the ECDSA end entity certificate
// being linted has Key Usage bits set other than digitalSignature,
// nonRepudiation/contentCommentment, and keyAgreement.
func (l *ecdsaInvalidKU) Execute(c *x509.Certificate) *lint.LintResult {
	// RFC 5480, Section 3 "Key Usage Bits" says:
	//
	//   If the keyUsage extension is present in an End Entity (EE)
	//   certificate that indicates id-ecPublicKey in SubjectPublicKeyInfo,
	//   then any combination of the following values MAY be present:
	//
	//     digitalSignature;
	//     nonRepudiation; and
	//     keyAgreement.
	//
	// So we set up `allowedKUs` to match. Note that per RFC 5280: recent editions
	// of X.509 renamed "nonRepudiation" to "contentCommitment", which is the name
	// of the Go x509 constant we use here alongside the digitalSignature and
	// keyAgreement constants.
	allowedKUs := map[x509.KeyUsage]bool{
		x509.KeyUsageDigitalSignature:  true,
		x509.KeyUsageContentCommitment: true,
		x509.KeyUsageKeyAgreement:      true,
	}

	var invalidKUs []string
	for ku, kuName := range util.KeyUsageToString {
		if c.KeyUsage&ku != 0 {
			if !allowedKUs[ku] {
				invalidKUs = append(invalidKUs, kuName)
			}
		}
	}

	if len(invalidKUs) > 0 {
		// Sort the invalid KUs to allow consistent ordering of Details messages for
		// unit testing
		sort.Strings(invalidKUs)
		return &lint.LintResult{
			Status: lint.Notice,
			Details: fmt.Sprintf(
				"Certificate had unexpected key usage(s): %s",
				strings.Join(invalidKUs, ", ")),
		}
	}

	return &lint.LintResult{
		Status: lint.Pass,
	}
}
