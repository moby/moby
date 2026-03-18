package cabf_ev

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

type evNoBiz struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ev_business_category_missing",
		Description:   "EV certificates must include businessCategory in subject",
		Citation:      "EVGs: 9.2.3",
		Source:        lint.CABFEVGuidelines,
		EffectiveDate: util.ZeroDate,
		Lint:          NewEvNoBiz,
	})
}

func NewEvNoBiz() lint.LintInterface {
	return &evNoBiz{}
}

func (l *evNoBiz) CheckApplies(c *x509.Certificate) bool {
	return util.IsEV(c.PolicyIdentifiers) && util.IsSubscriberCert(c)
}

func (l *evNoBiz) Execute(c *x509.Certificate) *lint.LintResult {
	if util.TypeInName(&c.Subject, util.BusinessOID) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
