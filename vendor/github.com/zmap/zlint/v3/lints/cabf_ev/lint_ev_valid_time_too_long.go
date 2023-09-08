package cabf_ev

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

type evValidTooLong struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ev_valid_time_too_long",
		Description:   "EV certificates must be 27 months in validity or less",
		Citation:      "EVGs 1.0: 8(a), EVGs 1.6.1: 9.4",
		Source:        lint.CABFEVGuidelines,
		EffectiveDate: util.ZeroDate,
		Lint:          &evValidTooLong{},
	})
}

func (l *evValidTooLong) Initialize() error {
	return nil
}

func (l *evValidTooLong) CheckApplies(c *x509.Certificate) bool {
	// CA/Browser Forum Ballot 193 changed the maximum validity period to be
	// 825 days, which is more permissive than 27-month certificates, as that
	// is 823 days.
	return c.NotBefore.Before(util.SubCert825Days) &&
		util.IsSubscriberCert(c) &&
		util.IsEV(c.PolicyIdentifiers)
}

func (l *evValidTooLong) Execute(c *x509.Certificate) *lint.LintResult {
	if c.NotBefore.AddDate(0, 27, 0).Before(c.NotAfter) {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
