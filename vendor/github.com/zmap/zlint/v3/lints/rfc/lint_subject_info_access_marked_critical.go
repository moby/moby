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

type siaCrit struct{}

/************************************************
The subject information access extension indicates how to access information and services for the subject of the certificate in which the extension appears. When the subject is a CA, information and services may include certificate validation services and CA policy data. When the subject is an end entity, the information describes the type of services offered and how to access them. In this case, the contents of this extension are defined in the protocol specifications for the supported services. This extension may be included in end entity or CA certificates. Conforming CAs MUST mark this extension as non-critical.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_info_access_marked_critical",
		Description:   "Conforming CAs MUST mark the Subject Info Access extension as non-critical",
		Citation:      "RFC 5280: 4.2.2.2",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          NewSiaCrit,
	})
}

func NewSiaCrit() lint.LintInterface {
	return &siaCrit{}
}

func (l *siaCrit) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.SubjectInfoAccessOID)
}

func (l *siaCrit) Execute(c *x509.Certificate) *lint.LintResult {
	sia := util.GetExtFromCert(c, util.SubjectInfoAccessOID)
	if sia.Critical {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
