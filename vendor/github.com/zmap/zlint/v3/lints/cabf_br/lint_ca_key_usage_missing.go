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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type caKeyUsageMissing struct{}

/************************************************
RFC 5280: 4.2.1.3
Conforming CAs MUST include this extension in certificates that
   contain public keys that are used to validate digital signatures on
   other public key certificates or CRLs.  When present, conforming CAs
   SHOULD mark this extension as critical.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ca_key_usage_missing",
		Description:   "Root and Subordinate CA certificate keyUsage extension MUST be present",
		Citation:      "BRs: 7.1.2.1, RFC 5280: 4.2.1.3",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.RFC3280Date,
		Lint:          NewCaKeyUsageMissing,
	})
}

func NewCaKeyUsageMissing() lint.LintInterface {
	return &caKeyUsageMissing{}
}

func (l *caKeyUsageMissing) CheckApplies(c *x509.Certificate) bool {
	return c.IsCA
}

func (l *caKeyUsageMissing) Execute(c *x509.Certificate) *lint.LintResult {
	if c.KeyUsage != x509.KeyUsage(0) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
