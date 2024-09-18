package rfc

/*
 * ZLint Copyright 2021 Regents of the University of Michigan
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

type checkKeyUsageCritical struct{}

// "When present, conforming CAs SHOULD mark this extension as critical."

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_ext_key_usage_not_critical",
		Description:   "The keyUsage extension SHOULD be critical",
		Citation:      "RFC 5280: 4.2.1.3",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          &checkKeyUsageCritical{},
	})
}

func (l *checkKeyUsageCritical) Initialize() error {
	return nil
}

func (l *checkKeyUsageCritical) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.KeyUsageOID)
}

func (l *checkKeyUsageCritical) Execute(c *x509.Certificate) *lint.LintResult {
	keyUsage := util.GetExtFromCert(c, util.KeyUsageOID)
	if keyUsage == nil {
		return &lint.LintResult{Status: lint.NA}
	}
	if keyUsage.Critical {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Warn}
	}
}
