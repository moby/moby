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
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type IANPubSuffix struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_ian_iana_pub_suffix_empty",
		Description:   "Domain SHOULD NOT have a bare public suffix",
		Citation:      "awslabs certlint",
		Source:        lint.Community,
		EffectiveDate: util.ZeroDate,
		Lint:          NewIANPubSuffix,
	})
}

func NewIANPubSuffix() lint.LintInterface {
	return &IANPubSuffix{}
}

func (l *IANPubSuffix) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.IssuerAlternateNameOID)
}

func (l *IANPubSuffix) Execute(c *x509.Certificate) *lint.LintResult {
	for _, dns := range c.IANDNSNames {
		if len(strings.Split(dns, ".")) < 3 {
			return &lint.LintResult{Status: lint.Warn}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
