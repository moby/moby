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
	"crypto/rsa"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type rsaParsedTestsKeySize struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_rsa_mod_less_than_2048_bits",
		Description:   "For certificates valid after 31 Dec 2013, all certificates using RSA public key algorithm MUST have 2048 bits of modulus",
		Citation:      "BRs: 6.1.5",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.ZeroDate,
		Lint:          NewRsaParsedTestsKeySize,
	})
}

func NewRsaParsedTestsKeySize() lint.LintInterface {
	return &rsaParsedTestsKeySize{}
}

func (l *rsaParsedTestsKeySize) CheckApplies(c *x509.Certificate) bool {
	_, ok := c.PublicKey.(*rsa.PublicKey)
	return ok && c.PublicKeyAlgorithm == x509.RSA && util.OnOrAfter(c.NotAfter, util.NoRSA1024Date)
}

func (l *rsaParsedTestsKeySize) Execute(c *x509.Certificate) *lint.LintResult {
	key := c.PublicKey.(*rsa.PublicKey)
	if key.N.BitLen() < 2048 {
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
