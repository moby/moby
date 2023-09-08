package rfc

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

type distribNoLDAPorURI struct{}

/************************************************
RFC 5280: 4.2.1.13
When present, DistributionPointName SHOULD include at least one LDAP or HTTP URI.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_distribution_point_missing_ldap_or_uri",
		Description:   "When present in the CRLDistributionPoints extension, DistributionPointName SHOULD include at least one LDAP or HTTP URI",
		Citation:      "RFC 5280: 4.2.1.13",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          &distribNoLDAPorURI{},
	})
}

func (l *distribNoLDAPorURI) Initialize() error {
	return nil
}

func (l *distribNoLDAPorURI) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.CrlDistOID)
}

func (l *distribNoLDAPorURI) Execute(c *x509.Certificate) *lint.LintResult {
	for _, point := range c.CRLDistributionPoints {
		if point = strings.ToLower(point); strings.HasPrefix(point, "http://") || strings.HasPrefix(point, "ldap://") {
			return &lint.LintResult{Status: lint.Pass}
		}
	}
	return &lint.LintResult{Status: lint.Warn}
}
