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
	"encoding/asn1"
	"time"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type generalizedPre2050 struct{}

/*********************************************************************
CAs conforming to this profile MUST always encode certificate
validity dates through the year 2049 as UTCTime; certificate validity
dates in 2050 or later MUST be encoded as GeneralizedTime.
Conforming applications MUST be able to process validity dates that
are encoded in either UTCTime or GeneralizedTime.
*********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_wrong_time_format_pre2050",
		Description:   "Certificates valid through the year 2049 MUST be encoded in UTC time",
		Citation:      "RFC 5280: 4.1.2.5",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          &generalizedPre2050{},
	})
}

func (l *generalizedPre2050) Initialize() error {
	return nil
}

func (l *generalizedPre2050) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *generalizedPre2050) Execute(c *x509.Certificate) *lint.LintResult {
	date1, date2 := util.GetTimes(c)
	var t time.Time
	type1, type2 := util.FindTimeType(date1, date2)
	if type1 == 24 {
		temp, err := asn1.Marshal(date1)
		if err != nil {
			return &lint.LintResult{Status: lint.Fatal}
		}
		_, err = asn1.Unmarshal(temp, &t)
		if err != nil {
			return &lint.LintResult{Status: lint.Fatal}
		}
		if t.Before(util.GeneralizedDate) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	if type2 == 24 {
		temp, err := asn1.Marshal(date2)
		if err != nil {
			return &lint.LintResult{Status: lint.Fatal}
		}
		_, err = asn1.Unmarshal(temp, &t)
		if err != nil {
			return &lint.LintResult{Status: lint.Fatal}
		}
		if t.Before(util.GeneralizedDate) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
