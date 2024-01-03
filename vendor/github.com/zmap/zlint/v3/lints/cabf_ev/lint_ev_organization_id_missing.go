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

package cabf_ev

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type evOrgIdExtMissing struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name: "e_ev_organization_id_missing",
		Description: "Effective January 31, 2020, if the subject:organizationIdentifier field is " +
			"present, this [cabfOrganizationIdentifier] field MUST be present.",
		Citation:      "CA/Browser Forum EV Guidelines v1.7.0, Sec. 9.8.2",
		Source:        lint.CABFEVGuidelines,
		EffectiveDate: util.CABFEV_9_8_2,
		Lint:          &evOrgIdExtMissing{},
	})
}

func (l *evOrgIdExtMissing) Initialize() error {
	return nil
}

func (l *evOrgIdExtMissing) CheckApplies(c *x509.Certificate) bool {
	return util.IsEV(c.PolicyIdentifiers) && len(c.Subject.OrganizationIDs) > 0
}

func (l *evOrgIdExtMissing) Execute(c *x509.Certificate) *lint.LintResult {
	if !util.IsExtInCert(c, util.CabfExtensionOrganizationIdentifier) {
		return &lint.LintResult{
			Status: lint.Error,
			Details: "subject:organizationIdentifier field is present in an EV certificate " +
				"but the CA/Browser Forum Organization Identifier Field Extension is missing"}
	}
	return &lint.LintResult{Status: lint.Pass}
}
