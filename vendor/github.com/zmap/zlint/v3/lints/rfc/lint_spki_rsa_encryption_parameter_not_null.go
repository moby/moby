package rfc

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
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type rsaSPKIEncryptionParamNotNULL struct{}

/*******************************************************************************************************
"RFC5280: RFC 4055, Section 1.2"
RSA: Encoded algorithm identifier MUST have NULL parameters.
*******************************************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_spki_rsa_encryption_parameter_not_null",
		Description:   "RSA: Encoded public key algorithm identifier MUST have NULL parameters",
		Citation:      "RFC 4055, Section 1.2",
		Source:        lint.RFC5280, // RFC4055 is referenced in lint.RFC5280, Section 1
		EffectiveDate: util.RFC5280Date,
		Lint:          NewRsaSPKIEncryptionParamNotNULL,
	})
}

func NewRsaSPKIEncryptionParamNotNULL() lint.LintInterface {
	return &rsaSPKIEncryptionParamNotNULL{}
}

func (l *rsaSPKIEncryptionParamNotNULL) CheckApplies(c *x509.Certificate) bool {
	// explicitly check for util.OidRSAEncryption, as RSA-PSS or RSA-OAEP certificates might be classified with c.PublicKeyAlgorithm = RSA
	return c.PublicKeyAlgorithmOID.Equal(util.OidRSAEncryption)
}

func (l *rsaSPKIEncryptionParamNotNULL) Execute(c *x509.Certificate) *lint.LintResult {
	encodedPublicKeyAid, err := util.GetPublicKeyAidEncoded(c)
	if err != nil {
		return &lint.LintResult{
			Status:  lint.Error,
			Details: fmt.Sprintf("error reading public key algorithm identifier: %v", err),
		}
	}

	if err := util.CheckAlgorithmIDParamNotNULL(encodedPublicKeyAid, util.OidRSAEncryption); err != nil {
		return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("certificate pkixPublicKey %s", err.Error())}
	}

	return &lint.LintResult{Status: lint.Pass}
}
