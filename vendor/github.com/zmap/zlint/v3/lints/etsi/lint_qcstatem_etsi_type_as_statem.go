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

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type qcStatemEtsiTypeAsStatem struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_qcstatem_etsi_type_as_statem",
		Description:   "Checks for erroneous QC Statement OID that actually are represented by ETSI ESI QC type OID.",
		Citation:      "ETSI EN 319 412 - 5 V2.2.1 (2017 - 11) / Section 4.2.3",
		Source:        lint.EtsiEsi,
		EffectiveDate: util.EtsiEn319_412_5_V2_2_1_Date,
		Lint:          &qcStatemEtsiTypeAsStatem{},
	})
}

func (l *qcStatemEtsiTypeAsStatem) Initialize() error {
	return nil
}

func (l *qcStatemEtsiTypeAsStatem) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.QcStateOid)
}

func (l *qcStatemEtsiTypeAsStatem) Execute(c *x509.Certificate) *lint.LintResult {
	errString := ""
	ext := util.GetExtFromCert(c, util.QcStateOid)

	oidList := make([]*asn1.ObjectIdentifier, 3)
	oidList[0] = &util.IdEtsiQcsQctEsign
	oidList[1] = &util.IdEtsiQcsQctEseal
	oidList[2] = &util.IdEtsiQcsQctWeb

	for _, oid := range oidList {
		r := util.ParseQcStatem(ext.Value, *oid)
		util.AppendToStringSemicolonDelim(&errString, r.GetErrorInfo())
		if r.IsPresent() {
			util.AppendToStringSemicolonDelim(&errString, fmt.Sprintf("ETSI QC Type OID %v used as QC statement", oid))
		}
	}

	if len(errString) == 0 {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error, Details: errString}
	}
}
