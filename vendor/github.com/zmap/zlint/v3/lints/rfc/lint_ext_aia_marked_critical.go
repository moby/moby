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

type ExtAiaMarkedCritical struct{}

/************************************************
Authority Information Access
   The authority information access extension indicates how to access information and services for the issuer of the certificate in which the extension appears. Information and services may include on-line validation services and CA policy data. (The location of CRLs is not specified in this extension; that information is provided by the cRLDistributionPoints extension.) This extension may be included in end entity or CA certificates. Conforming CAs MUST mark this extension as non-critical.
************************************************/
//See also: BRs: 7.1.2.3 & CAB: 7.1.2.2

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_aia_marked_critical",
		Description:   "Conforming CAs must mark the Authority Information Access extension as non-critical",
		Citation:      "RFC 5280: 4.2.2.1",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewExtAiaMarkedCritical,
	})
}

func NewExtAiaMarkedCritical() lint.LintInterface {
	return &ExtAiaMarkedCritical{}
}

func (l *ExtAiaMarkedCritical) CheckApplies(cert *x509.Certificate) bool {
	return util.IsExtInCert(cert, util.AiaOID)
}

func (l *ExtAiaMarkedCritical) Execute(cert *x509.Certificate) *lint.LintResult {
	if util.GetExtFromCert(cert, util.AiaOID).Critical {
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
