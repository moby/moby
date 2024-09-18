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

type caOrganizationNameMissing struct{}

/************************************************
BRs: 7.1.2.1e
The Certificate Subject MUST contain the following: organizationName (OID 2.5.4.10): This field MUST be present and the contents MUST contain either the Subject CA’s name or DBA as verified under Section 3.2.2.2.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ca_organization_name_missing",
		Description:   "Root and Subordinate CA certificates MUST have a organizationName present in subject information",
		Citation:      "BRs: 7.1.2.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          &caOrganizationNameMissing{},
	})
}

func (l *caOrganizationNameMissing) Initialize() error {
	return nil
}

func (l *caOrganizationNameMissing) CheckApplies(c *x509.Certificate) bool {
	return c.IsCA
}

func (l *caOrganizationNameMissing) Execute(c *x509.Certificate) *lint.LintResult {
	if c.Subject.Organization != nil && c.Subject.Organization[0] != "" {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
