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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type SANWildCardFirst struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_san_wildcard_not_first",
		Description:   "A wildcard MUST be in the first label of FQDN (ie not: www.*.com) (Only checks DNSName)",
		Citation:      "awslabs certlint",
		Source:        lint.Community,
		EffectiveDate: util.ZeroDate,
		Lint:          NewSANWildCardFirst,
	})
}

func NewSANWildCardFirst() lint.LintInterface {
	return &SANWildCardFirst{}
}

func (l *SANWildCardFirst) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.SubjectAlternateNameOID)
}

func (l *SANWildCardFirst) Execute(c *x509.Certificate) *lint.LintResult {
	for _, dns := range c.DNSNames {
		for i := 1; i < len(dns); i++ {
			if dns[i] == '*' {
				return &lint.LintResult{Status: lint.Error}
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
