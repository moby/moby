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
	"crypto/ecdsa"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type ecImproperCurves struct{}

/************************************************
BRs: 6.1.5
Certificates MUST meet the following requirements for algorithm type and key size.
ECC Curve: NIST P-256, P-384, or P-521
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:        "e_ec_improper_curves",
		Description: "Only one of NIST P‐256, P‐384, or P‐521 can be used",
		Citation:    "BRs: 6.1.5",
		Source:      lint.CABFBaselineRequirements,
		// Refer to BRs: 6.1.5, taking the statement "Before 31 Dec 2010" literally
		EffectiveDate: util.ZeroDate,
		Lint:          NewEcImproperCurves,
	})
}

func NewEcImproperCurves() lint.LintInterface {
	return &ecImproperCurves{}
}

func (l *ecImproperCurves) CheckApplies(c *x509.Certificate) bool {
	return c.PublicKeyAlgorithm == x509.ECDSA
}

func (l *ecImproperCurves) Execute(c *x509.Certificate) *lint.LintResult {
	/* Declare theKey to be a ECDSA Public Key */
	var theKey *ecdsa.PublicKey
	/* Need to do different things based on what c.PublicKey is */
	switch keyType := c.PublicKey.(type) {
	case *x509.AugmentedECDSA:
		theKey = keyType.Pub
	case *ecdsa.PublicKey:
		theKey = keyType
	}
	/* Now can actually check the params */
	theParams := theKey.Curve.Params()
	switch theParams.Name {
	case "P-256", "P-384", "P-521":
		return &lint.LintResult{Status: lint.Pass}
	default:
		return &lint.LintResult{Status: lint.Error}
	}
}
