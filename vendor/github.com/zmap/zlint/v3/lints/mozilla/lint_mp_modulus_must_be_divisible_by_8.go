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

type modulusDivisibleBy8 struct{}

/********************************************************************
Section 5.1 - Algorithms
RSA keys whose modulus size in bits is divisible by 8, and is at least 2048.
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_mp_modulus_must_be_divisible_by_8",
		Description:   "RSA keys must have a modulus size divisible by 8",
		Citation:      "Mozilla Root Store Policy / Section 5.1",
		Source:        lint.MozillaRootStorePolicy,
		EffectiveDate: util.MozillaPolicy24Date,
		Lint:          &modulusDivisibleBy8{},
	})
}

func (l *modulusDivisibleBy8) Initialize() error {
	return nil
}

func (l *modulusDivisibleBy8) CheckApplies(c *x509.Certificate) bool {
	return c.PublicKeyAlgorithm == x509.RSA
}

func (l *modulusDivisibleBy8) Execute(c *x509.Certificate) *lint.LintResult {
	pubKey, ok := c.PublicKey.(*rsa.PublicKey)
	if !ok {
		return &lint.LintResult{
			Status:  lint.Fatal,
			Details: "certificate public key was not an RSA public key",
		}
	}

	if bitLen := pubKey.N.BitLen(); (bitLen % 8) != 0 {
		return &lint.LintResult{Status: lint.Error}
	}

	return &lint.LintResult{Status: lint.Pass}
}
