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

package cabf_ev

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ev_san_ip_address_present",
		Description:   "The Subject Alternate Name extension MUST contain only 'dnsName' name types.",
		Citation:      "CABF EV Guidelines 1.7.8 Section 9.8.1",
		Source:        lint.CABFEVGuidelines,
		EffectiveDate: util.ZeroDate,
		Lint:          NewEvSanIpAddressPresent,
	})
}

type EvSanIpAddressPresent struct{}

func NewEvSanIpAddressPresent() lint.LintInterface {
	return &EvSanIpAddressPresent{}
}

func (l *EvSanIpAddressPresent) CheckApplies(c *x509.Certificate) bool {
	return util.IsEV(c.PolicyIdentifiers)
}

func (l *EvSanIpAddressPresent) Execute(c *x509.Certificate) *lint.LintResult {
	if len(c.IPAddresses) > 0 {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
