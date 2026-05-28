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

package etsi

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type qcStatemQcEtsiPresentQcsCritical struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_qcstatem_etsi_present_qcs_critical",
		Description:   "Checks that a QC Statement which contains any of the id-etsi-qcs-... QC Statements is not marked critical",
		Citation:      "ETSI EN 319 412 - 5 V2.2.1 (2017 - 11) / Section 4.1",
		Source:        lint.EtsiEsi,
		EffectiveDate: util.EtsiEn319_412_5_V2_2_1_Date,
		Lint:          NewQcStatemQcEtsiPresentQcsCritical,
	})
}

func NewQcStatemQcEtsiPresentQcsCritical() lint.LintInterface {
	return &qcStatemQcEtsiPresentQcsCritical{}
}

func (l *qcStatemQcEtsiPresentQcsCritical) CheckApplies(c *x509.Certificate) bool {
	if !util.IsExtInCert(c, util.QcStateOid) {
		return false
	}
	if util.IsAnyEtsiQcStatementPresent(util.GetExtFromCert(c, util.QcStateOid).Value) {
		return true
	}
	return false
}

func (l *qcStatemQcEtsiPresentQcsCritical) Execute(c *x509.Certificate) *lint.LintResult {
	errString := ""
	ext := util.GetExtFromCert(c, util.QcStateOid)
	if ext.Critical {
		errString = "ETSI QC Statement is present and QC Statements extension is marked critical"
	}

	if len(errString) == 0 {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error, Details: errString}
	}
}
