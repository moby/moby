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
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subjectCommonNameNotExactlyFromSAN struct{}

/************************************************
If present, this field MUST contain exactly one entry that is one of the values contained
in the Certificate's `subjectAltName` extension

If the [subject:commonName] is a Fully-Qualified Domain Name or Wildcard Domain Name, then
the value MUST be encoded as a character-for-character copy of the dNSName entry value from
the subjectAltName extension.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_common_name_not_exactly_from_san",
		Description:   "The common name field in subscriber certificates must include only names from the SAN extension",
		Citation:      "BRs: 7.1.4.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABFBRs_1_8_0_Date,
		Lint:          NewSubjectCommonNameNotExactlyFromSAN,
	})
}

func NewSubjectCommonNameNotExactlyFromSAN() lint.LintInterface {
	return &subjectCommonNameNotExactlyFromSAN{}
}

func (l *subjectCommonNameNotExactlyFromSAN) CheckApplies(c *x509.Certificate) bool {
	return len(c.Subject.CommonNames) > 0 && !util.IsCACert(c)
}

func (l *subjectCommonNameNotExactlyFromSAN) Execute(c *x509.Certificate) *lint.LintResult {
	for _, cn := range c.Subject.CommonNames {
		var cnFound = false
		for _, dn := range c.DNSNames {
			if cn == dn {
				cnFound = true
				break
			}
		}
		if cnFound {
			continue
		}

		for _, ip := range c.IPAddresses {
			if cn == ip.String() {
				cnFound = true
				break
			}
		}
		if cnFound {
			continue
		}

		return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("Missing common name, '%s'", cn)}
	}

	return &lint.LintResult{Status: lint.Pass}
}
