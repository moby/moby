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
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type qcStatemQcPdsValid struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_qcstatem_qcpds_valid",
		Description:   "Checks that a QC Statement of the type id-etsi-qcs-QcPDS has the correct form",
		Citation:      "ETSI EN 319 412 - 5 V2.2.1 (2017 - 11) / Section 4.3.4",
		Source:        lint.EtsiEsi,
		EffectiveDate: util.EtsiEn319_412_5_V2_2_1_Date,
		Lint:          &qcStatemQcPdsValid{},
	})
}

func (this *qcStatemQcPdsValid) getStatementOid() *asn1.ObjectIdentifier {
	return &util.IdEtsiQcsQcEuPDS
}

func (l *qcStatemQcPdsValid) Initialize() error {
	return nil
}

func (l *qcStatemQcPdsValid) CheckApplies(c *x509.Certificate) bool {
	if !util.IsExtInCert(c, util.QcStateOid) {
		return false
	}
	if util.ParseQcStatem(util.GetExtFromCert(c, util.QcStateOid).Value, *l.getStatementOid()).IsPresent() {
		return true
	}
	return false
}

func isInList(s string, list []string) bool {
	for _, i := range list {
		if strings.Compare(i, s) == 0 {
			return true
		}
	}
	return false
}

func (l *qcStatemQcPdsValid) Execute(c *x509.Certificate) *lint.LintResult {
	errString := ""
	ext := util.GetExtFromCert(c, util.QcStateOid)
	s := util.ParseQcStatem(ext.Value, *l.getStatementOid())
	errString += s.GetErrorInfo()
	if len(errString) == 0 {
		codeList := make([]string, 0)
		foundEn := false
		pds := s.(util.EtsiQcPds)
		if len(pds.PdsLocations) == 0 {
			util.AppendToStringSemicolonDelim(&errString, "PDS list is empty")
		}
		for i, loc := range pds.PdsLocations {
			if len(loc.Language) != 2 {
				util.AppendToStringSemicolonDelim(&errString, fmt.Sprintf("PDS location %d has a language code with an invalid length", i))
			}
			if strings.Compare(strings.ToLower(loc.Language), "en") == 0 {
				foundEn = true
			}
			if isInList(strings.ToLower(loc.Language), codeList) {
				util.AppendToStringSemicolonDelim(&errString, "country code '"+loc.Language+"' appears multiple times")
			}
			codeList = append(codeList, loc.Language)

		}
		if !foundEn {
			util.AppendToStringSemicolonDelim(&errString, "no english PDS present")
		}
	}
	if len(errString) == 0 {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error, Details: errString}
	}
}
