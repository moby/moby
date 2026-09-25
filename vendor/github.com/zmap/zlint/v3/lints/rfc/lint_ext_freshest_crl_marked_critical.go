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
	"github.com/zmap/zcrypto/x509/pkix"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type ExtFreshestCrlMarkedCritical struct{}

/************************************************
The freshest CRL extension identifies how delta CRL information is obtained. The extension MUST be marked as non-critical by conforming CAs. Further discussion of CRL management is contained in Section 5.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_freshest_crl_marked_critical",
		Description:   "Freshest CRL MUST be marked as non-critical by conforming CAs",
		Citation:      "RFC 5280: 4.2.1.15",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          NewExtFreshestCrlMarkedCritical,
	})
}

func NewExtFreshestCrlMarkedCritical() lint.LintInterface {
	return &ExtFreshestCrlMarkedCritical{}
}

func (l *ExtFreshestCrlMarkedCritical) CheckApplies(cert *x509.Certificate) bool {
	return util.IsExtInCert(cert, util.FreshCRLOID)
}

func (l *ExtFreshestCrlMarkedCritical) Execute(cert *x509.Certificate) *lint.LintResult {
	var fCRL *pkix.Extension = util.GetExtFromCert(cert, util.FreshCRLOID)
	if fCRL != nil && fCRL.Critical {
		return &lint.LintResult{Status: lint.Error}
	} else if fCRL != nil && !fCRL.Critical {
		return &lint.LintResult{Status: lint.Pass}
	}
	return &lint.LintResult{Status: lint.NA} //shouldn't happen
}
