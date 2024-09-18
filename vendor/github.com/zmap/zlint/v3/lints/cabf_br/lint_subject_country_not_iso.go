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
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type countryNotIso struct{}

/**************************************************************************************************************
BRs: 7.1.4.2.2
Certificate Field: issuer:countryName (OID 2.5.4.6)
Required/Optional: Required
Contents: This field MUST contain the two-letter ISO 3166-1 country code for the country in which the issuer’s
place of business is located.
**************************************************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_country_not_iso",
		Description:   "The country name field MUST contain the two-letter ISO code for the country or XX",
		Citation:      "BRs: 7.1.4.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          &countryNotIso{},
	})
}

func (l *countryNotIso) Initialize() error {
	return nil
}

func (l *countryNotIso) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *countryNotIso) Execute(c *x509.Certificate) *lint.LintResult {
	for _, j := range c.Subject.Country {
		if !util.IsISOCountryCode(strings.ToUpper(j)) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
