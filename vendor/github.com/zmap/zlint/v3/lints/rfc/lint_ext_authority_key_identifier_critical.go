package rfc

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

type authorityKeyIdCritical struct{}

/*********************************************************
RFC 5280: 4.2.1.1
Conforming CAs MUST mark this extension as non-critical.
**********************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_authority_key_identifier_critical",
		Description:   "The authority key identifier extension must be non-critical",
		Citation:      "RFC 5280: 4.2.1.1",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewAuthorityKeyIdCritical,
	})
}

func NewAuthorityKeyIdCritical() lint.LintInterface {
	return &authorityKeyIdCritical{}
}

func (l *authorityKeyIdCritical) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.AuthkeyOID)
}

func (l *authorityKeyIdCritical) Execute(c *x509.Certificate) *lint.LintResult {
	aki := util.GetExtFromCert(c, util.AuthkeyOID) //pointer to the extension
	if aki.Critical {
		return &lint.LintResult{Status: lint.Error}
	} else { //implies !aki.Critical
		return &lint.LintResult{Status: lint.Pass}
	}
}
