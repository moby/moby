package rfc

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

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type keyUsageCertSignNoCa struct{}

/************************************************************************
RFC 5280: 4.2.1.9
The cA boolean indicates whether the certified public key may be used
   to verify certificate signatures.  If the cA boolean is not asserted,
   then the keyCertSign bit in the key usage extension MUST NOT be
   asserted.  If the basic constraints extension is not present in a
   version 3 certificate, or the extension is present but the cA boolean
   is not asserted, then the certified public key MUST NOT be used to
   verify certificate signatures.
************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_key_usage_cert_sign_without_ca",
		Description:   "if the keyCertSign bit is asserted, then the cA bit in the basic constraints extension MUST also be asserted",
		Citation:      "RFC 5280: 4.2.1.3 & 4.2.1.9",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          &keyUsageCertSignNoCa{},
	})
}

func (l *keyUsageCertSignNoCa) Initialize() error {
	return nil
}

func (l *keyUsageCertSignNoCa) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.KeyUsageOID)
}

func (l *keyUsageCertSignNoCa) Execute(c *x509.Certificate) *lint.LintResult {
	if (c.KeyUsage & x509.KeyUsageCertSign) != 0 {
		if c.BasicConstraintsValid && util.IsCACert(c) { //CA certs may assert certificate signing usage
			return &lint.LintResult{Status: lint.Pass}
		} else {
			return &lint.LintResult{Status: lint.Error}
		}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
