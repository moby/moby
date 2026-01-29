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

type noticeRefPres struct{}

/********************************************************************
The user notice has two optional fields: the noticeRef field and the
explicitText field. Conforming CAs SHOULD NOT use the noticeRef
option.
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_ext_cert_policy_contains_noticeref",
		Description:   "Compliant certificates SHOULD NOT use the noticeRef option",
		Citation:      "RFC 5280: 4.2.1.4",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          NewNoticeRefPres,
	})
}

func NewNoticeRefPres() lint.LintInterface {
	return &noticeRefPres{}
}

func (l *noticeRefPres) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.CertPolicyOID)
}

func (l *noticeRefPres) Execute(c *x509.Certificate) *lint.LintResult {
	for _, firstLvl := range c.NoticeRefNumbers {
		for _, number := range firstLvl {
			if number != nil {
				return &lint.LintResult{Status: lint.Warn}
			}
		}
	}
	for _, firstLvl := range c.NoticeRefOrgnization {
		for _, org := range firstLvl {
			if len(org.Bytes) != 0 {
				return &lint.LintResult{Status: lint.Warn}
			}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
