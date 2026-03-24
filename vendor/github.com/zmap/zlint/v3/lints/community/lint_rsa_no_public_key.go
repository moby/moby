package community

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

type rsaParsedPubKeyExist struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_rsa_no_public_key",
		Description:   "The RSA public key should be present",
		Citation:      "awslabs certlint",
		Source:        lint.Community,
		EffectiveDate: util.ZeroDate,
		Lint:          NewRsaParsedPubKeyExist,
	})
}

func NewRsaParsedPubKeyExist() lint.LintInterface {
	return &rsaParsedPubKeyExist{}
}

func (l *rsaParsedPubKeyExist) CheckApplies(c *x509.Certificate) bool {
	return c.PublicKeyAlgorithm == x509.RSA
}

func (l *rsaParsedPubKeyExist) Execute(c *x509.Certificate) *lint.LintResult {
	_, ok := c.PublicKey.(*rsa.PublicKey)
	if !ok {
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
