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
	"fmt"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ev_not_wildcard",
		Description:   "Wildcard certificates are not allowed for EV Certificates except for those with .onion as the TLD.",
		Citation:      "CABF EV Guidelines 1.7.8 Section 9.8.1",
		Source:        lint.CABFEVGuidelines,
		EffectiveDate: util.OnionOnlyEVDate,
		Lint:          NewEvNotWildCard,
	})
}

type EvNotWildCard struct{}

func NewEvNotWildCard() lint.LintInterface {
	return &EvNotWildCard{}
}

func (l *EvNotWildCard) CheckApplies(c *x509.Certificate) bool {
	return util.IsEV(c.PolicyIdentifiers)
}

func (l *EvNotWildCard) Execute(c *x509.Certificate) *lint.LintResult {
	names := append(c.GetParsedDNSNames(false), c.GetParsedSubjectCommonName(false))
	for _, name := range names {
		if name.ParseError != nil {
			continue
		}
		if strings.Contains(name.DomainString, "*") && !strings.HasSuffix(name.DomainString, util.OnionTLD) {
			return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("'%s' appears to be a wildcard domain", name.DomainString)}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
