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

package mozilla

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type prohibitDSAUsage struct{}

/************************************************
https://www.mozilla.org/en-US/about/governance/policies/security-group/certs/policy/

Subsection 5.1 Algorithms
Root certificates in our root program, and any certificate which chains up to them, MUST use only algorithms and key sizes from the following set:

- RSA keys whose modulus size in bits is divisible by 8, and is at least 2048.
- ECDSA keys using one of the following curves:
    + P-256
    + P-384
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_prohibit_dsa_usage",
		Description:   "DSA is not an explicitly allowed signature algorithm, therefore it is forbidden.",
		Citation:      "Mozilla Root Store Policy / Section 5.1",
		Source:        lint.MozillaRootStorePolicy,
		EffectiveDate: util.MozillaPolicy241Date,
		Lint:          NewProhibitDSAUsage,
	})
}

func NewProhibitDSAUsage() lint.LintInterface {
	return &prohibitDSAUsage{}
}

func (l *prohibitDSAUsage) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *prohibitDSAUsage) Execute(c *x509.Certificate) *lint.LintResult {
	if c.PublicKeyAlgorithm == x509.DSA {
		return &lint.LintResult{Status: lint.Error}
	}

	return &lint.LintResult{Status: lint.Pass}
}
