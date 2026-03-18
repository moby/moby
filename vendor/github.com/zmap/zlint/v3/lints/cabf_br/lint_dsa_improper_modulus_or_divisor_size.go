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
	"github.com/zmap/zcrypto/dsa"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type dsaImproperSize struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_dsa_improper_modulus_or_divisor_size",
		Description:   "Certificates MUST meet the following requirements for DSA algorithm type and key size: L=2048 and N=224,256 or L=3072 and N=256",
		Citation:      "BRs v1.7.0: 6.1.5",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.ZeroDate,
		Lint:          NewDsaImproperSize,
	})
}

func NewDsaImproperSize() lint.LintInterface {
	return &dsaImproperSize{}
}

func (l *dsaImproperSize) CheckApplies(c *x509.Certificate) bool {
	return c.PublicKeyAlgorithm == x509.DSA
}

func (l *dsaImproperSize) Execute(c *x509.Certificate) *lint.LintResult {
	dsaKey, ok := c.PublicKey.(*dsa.PublicKey)
	if !ok {
		return &lint.LintResult{Status: lint.NA}
	}
	L := dsaKey.Parameters.P.BitLen()
	N := dsaKey.Parameters.Q.BitLen()
	if (L == 2048 && N == 224) || (L == 2048 && N == 256) || (L == 3072 && N == 256) {
		return &lint.LintResult{Status: lint.Pass}
	}
	return &lint.LintResult{Status: lint.Error}
}
