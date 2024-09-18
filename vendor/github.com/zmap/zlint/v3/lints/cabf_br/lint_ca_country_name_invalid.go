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

type caCountryNameInvalid struct{}

/************************************************
BRs: 7.1.2.1e
The	Certificate	Subject	MUST contain the following:
‐	countryName	(OID 2.5.4.6).
This field MUST	contain	the	two‐letter	ISO	3166‐1 country code	for	the country
in which the CA’s place	of business	is located.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ca_country_name_invalid",
		Description:   "Root and Subordinate CA certificates MUST have a two-letter country code specified in ISO 3166-1",
		Citation:      "BRs: 7.1.2.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          &caCountryNameInvalid{},
	})
}

func (l *caCountryNameInvalid) Initialize() error {
	return nil
}

func (l *caCountryNameInvalid) CheckApplies(c *x509.Certificate) bool {
	return c.IsCA
}

func (l *caCountryNameInvalid) Execute(c *x509.Certificate) *lint.LintResult {
	if c.Subject.Country != nil {
		for _, j := range c.Subject.Country {
			if !util.IsISOCountryCode(j) {
				return &lint.LintResult{Status: lint.Error}
			}
		}
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.NA}
	}
}
