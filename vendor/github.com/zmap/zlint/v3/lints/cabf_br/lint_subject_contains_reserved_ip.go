package cabf_br

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
	"net"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subjectReservedIP struct{}

/************************************************
BRs: 7.1.4.2.1
Also as of the Effective Date, the CA SHALL NOT
issue a certificate with an Expiry Date later than
1 November 2015 with a subjectAlternativeName extension
or Subject commonName field containing a Reserved IP
Address or Internal Name.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_contains_reserved_ip",
		Description:   "Certificates expiring later than 11 Jan 2015 MUST NOT contain a reserved IP address in the common name field",
		Citation:      "BRs: 7.1.4.2.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewSubjectReservedIP,
	})
}

func NewSubjectReservedIP() lint.LintInterface {
	return &subjectReservedIP{}
}

func (l *subjectReservedIP) CheckApplies(c *x509.Certificate) bool {
	return c.NotAfter.After(util.NoReservedIP)
}

func (l *subjectReservedIP) Execute(c *x509.Certificate) *lint.LintResult {
	if ip := net.ParseIP(c.Subject.CommonName); ip != nil && util.IsIANAReserved(ip) {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
