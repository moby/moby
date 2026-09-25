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

type rootCAKeyUsagePresent struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_root_ca_key_usage_present",
		Description:   "Root CA certificates MUST have Key Usage Extension Present",
		Citation:      "BRs: 7.1.2.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewRootCAKeyUsagePresent,
	})
}

func NewRootCAKeyUsagePresent() lint.LintInterface {
	return &rootCAKeyUsagePresent{}
}

func (l *rootCAKeyUsagePresent) CheckApplies(c *x509.Certificate) bool {
	return util.IsRootCA(c)
}

func (l *rootCAKeyUsagePresent) Execute(c *x509.Certificate) *lint.LintResult {
	if util.IsExtInCert(c, util.KeyUsageOID) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
