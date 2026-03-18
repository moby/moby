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

type IANDNSNotIA5String struct{}

/********************************************************************
RFC 5280: 4.2.1.7
When the subjectAltName extension contains a domain name system
label, the domain name MUST be stored in the DNSName (an IA5String).
The name MUST be in the "preferred name syntax", as specified by
Section 3.5 of [RFC1034] and as modified by Section 2.1 of
[RFC1123].  Note that while uppercase and lowercase letters are
allowed in domain names, no significance is attached to the case.  In
addition, while the string " " is a legal domain name, subjectAltName
extensions with a DNSName of " " MUST NOT be used.  Finally, the use
of the DNS representation for Internet mail addresses
(subscriber.example.com instead of subscriber@example.com) MUST NOT
be used; such identities are to be encoded as rfc822Name.  Rules for
encoding internationalized domain names are specified in Section 7.2.
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_ian_dns_not_ia5_string",
		Description:   "DNSNames MUST be IA5 strings",
		Citation:      "RFC 5280: 4.2.1.7",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewIANDNSNotIA5String,
	})
}

func NewIANDNSNotIA5String() lint.LintInterface {
	return &IANDNSNotIA5String{}
}

func (l *IANDNSNotIA5String) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.IssuerAlternateNameOID)
}

func (l *IANDNSNotIA5String) Execute(c *x509.Certificate) *lint.LintResult {
	ext := util.GetExtFromCert(c, util.IssuerAlternateNameOID)
	if ext == nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	ok, err := util.AllAlternateNameWithTagAreIA5(ext, util.DNSNameTag)
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	if ok {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
