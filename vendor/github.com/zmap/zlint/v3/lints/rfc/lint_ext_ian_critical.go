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

type ExtIANCritical struct{}

/************************************************
Issuer Alternative Name
   As with Section 4.2.1.6, this extension is used to associate Internet style identities with the certificate issuer. Issuer alternative name MUST be encoded as in 4.2.1.6.  Issuer alternative names are not processed as part of the certification path validation algorithm in Section 6. (That is, issuer alternative names are not used in name chaining and name constraints are not enforced.)
   Where present, conforming CAs SHOULD mark this extension as non-critical.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_ext_ian_critical",
		Description:   "Issuer alternate name should be marked as non-critical",
		Citation:      "RFC 5280: 4.2.1.7",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewExtIANCritical,
	})
}

func NewExtIANCritical() lint.LintInterface {
	return &ExtIANCritical{}
}

func (l *ExtIANCritical) CheckApplies(cert *x509.Certificate) bool {
	return util.IsExtInCert(cert, util.IssuerAlternateNameOID)
}

func (l *ExtIANCritical) Execute(cert *x509.Certificate) *lint.LintResult {
	if util.GetExtFromCert(cert, util.IssuerAlternateNameOID).Critical {
		return &lint.LintResult{Status: lint.Warn}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
