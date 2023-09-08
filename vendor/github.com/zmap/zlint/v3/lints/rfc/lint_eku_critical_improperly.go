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

type ekuBadCritical struct{}

/************************************************
RFC 5280: 4.2.1.12
If a CA includes extended key usages to satisfy such applications,
   but does not wish to restrict usages of the key, the CA can include
   the special KeyPurposeId anyExtendedKeyUsage in addition to the
   particular key purposes required by the applications.  Conforming CAs
   SHOULD NOT mark this extension as critical if the anyExtendedKeyUsage
   KeyPurposeId is present.  Applications that require the presence of a
   particular purpose MAY reject certificates that include the
   anyExtendedKeyUsage OID but not the particular OID expected for the
   application.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_eku_critical_improperly",
		Description:   "Conforming CAs SHOULD NOT mark extended key usage extension as critical if the anyExtendedKeyUsage KeyPurposedID is present",
		Citation:      "RFC 5280: 4.2.1.12",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          &ekuBadCritical{},
	})
}

func (l *ekuBadCritical) Initialize() error {
	return nil
}

func (l *ekuBadCritical) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.EkuSynOid)
}

func (l *ekuBadCritical) Execute(c *x509.Certificate) *lint.LintResult {
	if e := util.GetExtFromCert(c, util.EkuSynOid); e.Critical {
		for _, single_use := range c.ExtKeyUsage {
			if single_use == x509.ExtKeyUsageAny {
				return &lint.LintResult{Status: lint.Warn}
			}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
