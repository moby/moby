package cabf_br

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

type dnsNameContainsBareIANASuffix struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_dnsname_contains_bare_iana_suffix",
		Description:   "DNSNames should not contain a bare IANA suffix.",
		Citation:      "BRs: 1.6.1, Base Domain Name",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          &dnsNameContainsBareIANASuffix{},
	})
}

func (l *dnsNameContainsBareIANASuffix) Initialize() error {
	return nil
}

func (l *dnsNameContainsBareIANASuffix) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func (l *dnsNameContainsBareIANASuffix) Execute(c *x509.Certificate) *lint.LintResult {
	if c.Subject.CommonName != "" && !util.CommonNameIsIP(c) {
		if util.IsInTLDMap(c.Subject.CommonName) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	for _, dns := range c.DNSNames {
		if util.IsInTLDMap(dns) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
