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
	"math/big"

	"github.com/zmap/zcrypto/dsa"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type dsaUniqueCorrectRepresentation struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_dsa_unique_correct_representation",
		Description:   "DSA: Public key value has the unique correct representation in the field, and that the key has the correct order in the subgroup",
		Citation:      "BRs v1.7.0: 6.1.6",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewDsaUniqueCorrectRepresentation,
	})
}

func NewDsaUniqueCorrectRepresentation() lint.LintInterface {
	return &dsaUniqueCorrectRepresentation{}
}

func (l *dsaUniqueCorrectRepresentation) CheckApplies(c *x509.Certificate) bool {
	return c.PublicKeyAlgorithm == x509.DSA
}

func (l *dsaUniqueCorrectRepresentation) Execute(c *x509.Certificate) *lint.LintResult {
	dsaKey, ok := c.PublicKey.(*dsa.PublicKey)
	if !ok {
		return &lint.LintResult{Status: lint.NA}
	}
	// Verify that 2 ≤ y ≤ p-2.
	two := big.NewInt(2)
	pMinusTwo := big.NewInt(0)
	pMinusTwo.Sub(dsaKey.P, two)
	if two.Cmp(dsaKey.Y) > 0 || dsaKey.Y.Cmp(pMinusTwo) > 0 {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
