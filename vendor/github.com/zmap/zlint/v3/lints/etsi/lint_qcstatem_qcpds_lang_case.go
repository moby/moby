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
	"fmt"
	"unicode"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type qcStatemQcPdsLangCase struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_qcstatem_qcpds_lang_case",
		Description:   "Checks that a QC Statement of the type id-etsi-qcs-QcPDS features a language code comprised of only lower case letters",
		Citation:      "ETSI EN 319 412 - 5 V2.2.1 (2017 - 11) / Section 4.3.4",
		Source:        lint.EtsiEsi,
		EffectiveDate: util.EtsiEn319_412_5_V2_2_1_Date,
		Lint:          &qcStatemQcPdsLangCase{},
	})
}

func (this *qcStatemQcPdsLangCase) getStatementOid() *asn1.ObjectIdentifier {
	return &util.IdEtsiQcsQcEuPDS
}

func (l *qcStatemQcPdsLangCase) Initialize() error {
	return nil
}

func (l *qcStatemQcPdsLangCase) CheckApplies(c *x509.Certificate) bool {
	if !util.IsExtInCert(c, util.QcStateOid) {
		return false
	}
	if util.ParseQcStatem(util.GetExtFromCert(c, util.QcStateOid).Value, *l.getStatementOid()).IsPresent() {
		return true
	}
	return false
}

func isOnlyLowerCaseLetters(s string) bool {
	for _, c := range s {
		if !unicode.IsLower(c) {
			return false
		}
	}
	return true
}

func (l *qcStatemQcPdsLangCase) Execute(c *x509.Certificate) *lint.LintResult {
	errString := ""
	wrnString := ""
	ext := util.GetExtFromCert(c, util.QcStateOid)
	s := util.ParseQcStatem(ext.Value, *l.getStatementOid())
	errString += s.GetErrorInfo()
	if len(errString) == 0 {
		pds := s.(util.EtsiQcPds)
		for i, loc := range pds.PdsLocations {
			if !isOnlyLowerCaseLetters(loc.Language) {
				util.AppendToStringSemicolonDelim(&wrnString, fmt.Sprintf("PDS location %d has a language code containing invalid letters", i))
			}

		}
	}
	if len(errString) == 0 {
		if len(wrnString) == 0 {
			return &lint.LintResult{Status: lint.Pass}
		} else {
			return &lint.LintResult{Status: lint.Warn, Details: wrnString}
		}
	} else {
		return &lint.LintResult{Status: lint.Error, Details: errString}
	}
}
