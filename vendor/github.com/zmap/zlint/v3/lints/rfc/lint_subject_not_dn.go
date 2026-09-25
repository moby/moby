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
	"reflect"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zcrypto/x509/pkix"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subjectDN struct{}

/*************************************************************************
 RFC 5280: 4.1.2.6
 Where it is non-empty, the subject field MUST contain an X.500
   distinguished name (DN). The DN MUST be unique for each subject
   entity certified by the one CA as defined by the issuer name field. A
   CA may issue more than one certificate with the same DN to the same
   subject entity.
*************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_not_dn",
		Description:   "When not empty, the subject field MUST be a distinguished name",
		Citation:      "RFC 5280: 4.1.2.6",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewSubjectDN,
	})
}

func NewSubjectDN() lint.LintInterface {
	return &subjectDN{}
}

func (l *subjectDN) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *subjectDN) Execute(c *x509.Certificate) *lint.LintResult {
	if reflect.TypeOf(c.Subject) != reflect.TypeOf(*(new(pkix.Name))) {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
