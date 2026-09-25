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

// CHANGE THIS COMMENT TO MATCH SOURCE TEXT

import (
	"crypto/rsa"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subCaModSize struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:        "e_old_sub_ca_rsa_mod_less_than_1024_bits",
		Description: "In a validity period beginning on or before 31 Dec 2010 and ending on or before 31 Dec 2013, subordinate CA certificates using RSA public key algorithm MUST use a 1024 bit modulus",
		Citation:    "BRs: 6.1.5",
		Source:      lint.CABFBaselineRequirements,
		// since effective date should be checked against end date in this specific case, putting time check into checkApplies instead, ZeroDate here to automatically pass NE test
		EffectiveDate: util.ZeroDate,
		Lint:          NewSubCaModSize,
	})
}

func NewSubCaModSize() lint.LintInterface {
	return &subCaModSize{}
}

func (l *subCaModSize) CheckApplies(c *x509.Certificate) bool {
	issueDate := c.NotBefore
	endDate := c.NotAfter
	_, ok := c.PublicKey.(*rsa.PublicKey)
	return ok && util.IsSubCA(c) && issueDate.Before(util.NoRSA1024RootDate) && endDate.Before(util.NoRSA1024Date)
}

func (l *subCaModSize) Execute(c *x509.Certificate) *lint.LintResult {
	key := c.PublicKey.(*rsa.PublicKey)
	if key.N.BitLen() < 1024 {
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
