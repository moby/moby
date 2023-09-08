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

package mozilla

import (
	"time"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type allowedEKU struct{}

/********************************************************************
Section 5.3 - Intermediate Certificates
Intermediate certificates created after January 1, 2019, with the exception
of cross-certificates that share a private key with a corresponding root
certificate: MUST contain an EKU extension; and, MUST NOT include the
anyExtendedKeyUsage KeyPurposeId; and, * MUST NOT include both the
id-kp-serverAuth and id-kp-emailProtection KeyPurposeIds in the same
certificate.
Note that the lint cannot distinguish cross-certificates from other
intermediates.
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "n_mp_allowed_eku",
		Description:   "A SubCA certificate must not have key usage that allows for both server auth and email protection, and must not use anyKeyUsage",
		Citation:      "Mozilla Root Store Policy / Section 5.3",
		Source:        lint.MozillaRootStorePolicy,
		EffectiveDate: time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC),
		Lint:          &allowedEKU{},
	})
}

func (l *allowedEKU) Initialize() error {
	return nil
}

func (l *allowedEKU) CheckApplies(c *x509.Certificate) bool {
	// TODO(@cpu): This lint should be limited to SubCAs that do not share
	// a private key with a corresponding root certificate in the Mozilla root
	// store. See https://github.com/zmap/zlint/issues/352
	return util.IsSubCA(c)
}

func (l *allowedEKU) Execute(c *x509.Certificate) *lint.LintResult {
	noEKU := len(c.ExtKeyUsage) == 0
	anyEKU := util.HasEKU(c, x509.ExtKeyUsageAny)
	emailAndServerAuthEKU :=
		util.HasEKU(c, x509.ExtKeyUsageEmailProtection) &&
			util.HasEKU(c, x509.ExtKeyUsageServerAuth)

	if noEKU || anyEKU || emailAndServerAuthEKU {
		// NOTE(@cpu): When this lint's scope is improved (see CheckApplies TODO)
		// this should be a lint.Error result instead of lint.Notice. See
		// https://github.com/zmap/zlint/issues/352
		return &lint.LintResult{Status: lint.Notice}
	}

	return &lint.LintResult{Status: lint.Pass}
}
