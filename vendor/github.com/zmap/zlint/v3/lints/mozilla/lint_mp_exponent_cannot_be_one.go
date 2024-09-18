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

package mozilla

import (
	"crypto/rsa"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type exponentCannotBeOne struct{}

/********************************************************************
Section 5.2 - Forbidden and Required Practices
CAs MUST NOT issue certificates that have:
- invalid public keys (e.g., RSA certificates with public exponent equal to 1);
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_mp_exponent_cannot_be_one",
		Description:   "CAs MUST NOT issue certificates that have invalid public keys (e.g., RSA certificates with public exponent equal to 1)",
		Citation:      "Mozilla Root Store Policy / Section 5.2",
		Source:        lint.MozillaRootStorePolicy,
		EffectiveDate: util.MozillaPolicy24Date,
		Lint:          &exponentCannotBeOne{},
	})
}

func (l *exponentCannotBeOne) Initialize() error {
	return nil
}

func (l *exponentCannotBeOne) CheckApplies(c *x509.Certificate) bool {
	return c.PublicKeyAlgorithm == x509.RSA
}

func (l *exponentCannotBeOne) Execute(c *x509.Certificate) *lint.LintResult {
	pubKey, ok := c.PublicKey.(*rsa.PublicKey)
	if !ok {
		return &lint.LintResult{
			Status:  lint.Fatal,
			Details: "certificate public key was not an RSA public key",
		}
	}

	if pubKey.E == 1 {
		return &lint.LintResult{Status: lint.Error}
	}

	return &lint.LintResult{Status: lint.Pass}
}
