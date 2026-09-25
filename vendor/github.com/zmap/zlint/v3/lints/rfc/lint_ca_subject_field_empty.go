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

type caSubjectEmpty struct{}

/************************************************
RFC 5280: 4.1.2.6
The subject field identifies the entity associated with the public
   key stored in the subject public key field.  The subject name MAY be
   carried in the subject field and/or the subjectAltName extension.  If
   the subject is a CA (e.g., the basic constraints extension, as
   discussed in Section 4.2.1.9, is present and the value of cA is
   TRUE), then the subject field MUST be populated with a non-empty
   distinguished name matching the contents of the issuer field (Section
   4.1.2.4) in all certificates issued by the subject CA.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ca_subject_field_empty",
		Description:   "CA Certificates subject field MUST not be empty and MUST have a non-empty distinguished name",
		Citation:      "RFC 5280: 4.1.2.6",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewCaSubjectEmpty,
	})
}

func NewCaSubjectEmpty() lint.LintInterface {
	return &caSubjectEmpty{}
}

func (l *caSubjectEmpty) CheckApplies(c *x509.Certificate) bool {
	return c.IsCA
}

func (l *caSubjectEmpty) Execute(c *x509.Certificate) *lint.LintResult {
	if util.NotAllNameFieldsAreEmpty(&c.Subject) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
