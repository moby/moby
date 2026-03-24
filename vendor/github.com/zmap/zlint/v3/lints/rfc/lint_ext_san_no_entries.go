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

type SANNoEntry struct{}

/**********************************************************************
RFC 5280: 4.2.1.6
If the subjectAltName extension is present, the sequence MUST contain
   at least one entry.  Unlike the subject field, conforming CAs MUST
   NOT issue certificates with subjectAltNames containing empty
   GeneralName fields.  For example, an rfc822Name is represented as an
   IA5String.  While an empty string is a valid IA5String, such an
   rfc822Name is not permitted by this profile.  The behavior of clients
   that encounter such a certificate when processing a certification
   path is not defined by this profile.
***********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_san_no_entries",
		Description:   "If present, the SAN extension MUST contain at least one entry",
		Citation:      "RFC 5280: 4.2.1.6",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewSANNoEntry,
	})
}

func NewSANNoEntry() lint.LintInterface {
	return &SANNoEntry{}
}

func (l *SANNoEntry) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.SubjectAlternateNameOID)
}

func (l *SANNoEntry) Execute(c *x509.Certificate) *lint.LintResult {
	san := util.GetExtFromCert(c, util.SubjectAlternateNameOID)
	if util.IsEmptyASN1Sequence(san.Value) {
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
