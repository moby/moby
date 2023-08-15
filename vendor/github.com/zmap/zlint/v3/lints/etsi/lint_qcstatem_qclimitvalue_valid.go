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

package etsi

import (
	"encoding/asn1"
	"unicode"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type qcStatemQcLimitValueValid struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_qcstatem_qclimitvalue_valid",
		Description:   "Checks that a QC Statement of the type id-etsi-qcs-QcLimitValue has the correct form",
		Citation:      "ETSI EN 319 412 - 5 V2.2.1 (2017 - 11) / Section 4.3.2",
		Source:        lint.EtsiEsi,
		EffectiveDate: util.EtsiEn319_412_5_V2_2_1_Date,
		Lint:          &qcStatemQcLimitValueValid{},
	})
}

func (this *qcStatemQcLimitValueValid) getStatementOid() *asn1.ObjectIdentifier {
	return &util.IdEtsiQcsQcLimitValue
}

func (l *qcStatemQcLimitValueValid) Initialize() error {
	return nil
}

func (l *qcStatemQcLimitValueValid) CheckApplies(c *x509.Certificate) bool {
	if !util.IsExtInCert(c, util.QcStateOid) {
		return false
	}
	if util.ParseQcStatem(util.GetExtFromCert(c, util.QcStateOid).Value, *l.getStatementOid()).IsPresent() {
		return true
	}
	return false
}

func isOnlyLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func (l *qcStatemQcLimitValueValid) Execute(c *x509.Certificate) *lint.LintResult {

	errString := ""
	ext := util.GetExtFromCert(c, util.QcStateOid)
	s := util.ParseQcStatem(ext.Value, *l.getStatementOid())
	errString += s.GetErrorInfo()
	if len(errString) == 0 {
		qcLv, ok := s.(util.EtsiQcLimitValue)
		if !ok {
			return &lint.LintResult{Status: lint.Error, Details: "parsed QcStatem is not a EtsiQcLimitValue"}
		}
		if qcLv.Amount < 0 {
			util.AppendToStringSemicolonDelim(&errString, "amount is negative")
		}
		if qcLv.IsNum {
			if qcLv.CurrencyNum < 1 || qcLv.CurrencyNum > 999 {
				util.AppendToStringSemicolonDelim(&errString, "numeric currency code is out of range")
			}
		} else {
			if len(qcLv.CurrencyAlph) != 3 {
				util.AppendToStringSemicolonDelim(&errString, "invalid string length of currency code")
			}
			if !isOnlyLetters(qcLv.CurrencyAlph) {
				util.AppendToStringSemicolonDelim(&errString, "currency code string contains not only letters")
			}

		}

	}
	if len(errString) == 0 {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error, Details: errString}
	}
}
