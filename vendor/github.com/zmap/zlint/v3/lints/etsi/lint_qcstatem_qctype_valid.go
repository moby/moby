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
	"fmt"

	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type qcStatemQctypeValid struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_qcstatem_qctype_valid",
		Description:   "Checks that a QC Statement of the type Id-etsi-qcs-QcType features a non-empty list of only the allowed QcType OIDs",
		Citation:      "ETSI EN 319 412 - 5 V2.2.1 (2017 - 11) / Section 4.2.3",
		Source:        lint.EtsiEsi,
		EffectiveDate: util.EtsiEn319_412_5_V2_2_1_Date,
		Lint:          NewQcStatemQctypeValid,
	})
}

func NewQcStatemQctypeValid() lint.LintInterface {
	return &qcStatemQctypeValid{}
}

func (this *qcStatemQctypeValid) getStatementOid() *asn1.ObjectIdentifier {
	return &util.IdEtsiQcsQcType
}

func (l *qcStatemQctypeValid) CheckApplies(c *x509.Certificate) bool {
	if !util.IsExtInCert(c, util.QcStateOid) {
		return false
	}
	if util.ParseQcStatem(util.GetExtFromCert(c, util.QcStateOid).Value, *l.getStatementOid()).IsPresent() {
		return true
	}
	return false
}

func (l *qcStatemQctypeValid) Execute(c *x509.Certificate) *lint.LintResult {

	errString := ""
	ext := util.GetExtFromCert(c, util.QcStateOid)
	s := util.ParseQcStatem(ext.Value, *l.getStatementOid())
	errString += s.GetErrorInfo()
	if len(errString) == 0 {
		qcType := s.(util.Etsi423QcType)
		if len(qcType.TypeOids) == 0 {
			errString += "no QcType present, sequence of OIDs is empty"
		}
		for _, t := range qcType.TypeOids {

			if !t.Equal(util.IdEtsiQcsQctEsign) && !t.Equal(util.IdEtsiQcsQctEseal) && !t.Equal(util.IdEtsiQcsQctWeb) {
				if len(errString) > 0 {
					errString += "; "
				}
				errString += fmt.Sprintf("encountered invalid ETSI QcType OID: %v", t)
			}
		}
	}

	if len(errString) == 0 {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error, Details: errString}
	}
}
