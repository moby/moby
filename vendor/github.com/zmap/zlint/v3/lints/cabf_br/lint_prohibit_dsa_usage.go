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

package cabf_br

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type prohibitDSAUsage struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_br_prohibit_dsa_usage",
		Description:   "DSA was removed from the Baseline Requirements as a valid signature algorithm in 1.7.1.",
		Citation:      "BRs: v1.7.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABFBRs_1_7_1_Date,
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
