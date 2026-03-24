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

package rfc

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type rsaAllowedKUCaNoEncipherment struct{}

/************************************************
RFC 3279: 2.3.1  RSA Keys
   If the keyUsage extension is present in a CA or CRL issuer
   certificate which conveys an RSA public key, any combination of the
   following values MAY be present:

      digitalSignature;
      nonRepudiation;
      keyEncipherment;
      dataEncipherment;
      keyCertSign; and
      cRLSign.

   However, this specification RECOMMENDS that if keyCertSign or cRLSign
   is present, both keyEncipherment and dataEncipherment SHOULD NOT be
   present.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_rsa_allowed_ku_no_encipherment_ca",
		Description:   "If Key usage value keyCertSign or cRLSign is present in a CA certificate both keyEncipherment and dataEncipherment SHOULD NOT be present",
		Citation:      "RFC 3279: 2.3.1",
		Source:        lint.RFC3279,
		EffectiveDate: util.RFC3279Date,
		Lint:          NewRsaAllowedKUCaNoEncipherment,
	})
}

func NewRsaAllowedKUCaNoEncipherment() lint.LintInterface {
	return &rsaAllowedKUCaNoEncipherment{}
}

func (l *rsaAllowedKUCaNoEncipherment) CheckApplies(c *x509.Certificate) bool {
	return c.PublicKeyAlgorithm == x509.RSA && util.HasKeyUsageOID(c) && util.IsCACert(c)
}

func (l *rsaAllowedKUCaNoEncipherment) Execute(c *x509.Certificate) *lint.LintResult {

	if util.HasKeyUsage(c, x509.KeyUsageCertSign) || util.HasKeyUsage(c, x509.KeyUsageCRLSign) {
		if util.HasKeyUsage(c, x509.KeyUsageKeyEncipherment) || util.HasKeyUsage(c, x509.KeyUsageDataEncipherment) {
			return &lint.LintResult{Status: lint.Error, Details: "CA certificate with an RSA key and key usage keyCertSign and/or cRLSign has additionally keyEncipherment and/or dataEncipherment key usage"}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
