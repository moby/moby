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
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type IANEmail struct{}

/************************************************************************
RFC 5280: 4.2.1.6
 When the issuerAltName extension contains an Internet mail address,
   the address MUST be stored in the rfc822Name.  The format of an
   rfc822Name is a "Mailbox" as defined in Section 4.1.2 of [RFC2821].
   A Mailbox has the form "Local-part@Domain".  Note that a Mailbox has
   no phrase (such as a common name) before it, has no comment (text
   surrounded in parentheses) after it, and is not surrounded by "<" and
   ">".  Rules for encoding Internet mail addresses that include
   internationalized domain names are specified in Section 7.5.
************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_ian_rfc822_format_invalid",
		Description:   "Email must not be surrounded with `<>`, and there MUST NOT be trailing comments in `()`",
		Citation:      "RFC 5280: 4.2.1.7",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewIANEmail,
	})
}

func NewIANEmail() lint.LintInterface {
	return &IANEmail{}
}

func (l *IANEmail) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.IssuerAlternateNameOID)
}

func (l *IANEmail) Execute(c *x509.Certificate) *lint.LintResult {
	for _, str := range c.IANEmailAddresses {
		if str == "" {
			continue
		}
		if strings.Contains(str, " ") {
			return &lint.LintResult{Status: lint.Error}
		} else if str[0] == '<' || str[len(str)-1] == ')' {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
