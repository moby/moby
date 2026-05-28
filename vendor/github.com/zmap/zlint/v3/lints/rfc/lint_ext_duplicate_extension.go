package rfc

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

import (
	"fmt"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type extDuplicateExtension struct{}

/************************************************
"A certificate MUST NOT include more than one instance of a particular extension."
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_duplicate_extension",
		Description:   "A certificate MUST NOT include more than one instance of a particular extension",
		Citation:      "RFC 5280: 4.2",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewExtDuplicateExtension,
	})
}

func NewExtDuplicateExtension() lint.LintInterface {
	return &extDuplicateExtension{}
}

func (l *extDuplicateExtension) CheckApplies(cert *x509.Certificate) bool {
	return cert.Version == 3
}

func (l *extDuplicateExtension) Execute(cert *x509.Certificate) *lint.LintResult {
	// Make two maps: one for all of the extensions in the cert, and one for any
	// OIDs that are found more than once.
	extensionOIDs := make(map[string]bool)
	duplicateOIDs := make(map[string]bool)

	// Iterate through the certificate extensions and update the maps.
	for _, ext := range cert.Extensions {
		// We can't use the `asn1.ObjectIdentifier` as a key (it's an int slice) so use
		// the str representation.
		oid := ext.Id.String()

		if alreadySeen := extensionOIDs[oid]; alreadySeen {
			duplicateOIDs[oid] = true
		} else {
			extensionOIDs[oid] = true
		}
	}

	// If there were no duplicates we're done, the cert passes.
	if len(duplicateOIDs) == 0 {
		return &lint.LintResult{Status: lint.Pass}
	}

	// If there were duplicates turn the map keys into a list so we
	// can join them for the details string.
	var duplicateOIDsList []string
	for oid := range duplicateOIDs {
		duplicateOIDsList = append(duplicateOIDsList, oid)
	}

	return &lint.LintResult{
		Status: lint.Error,
		Details: fmt.Sprintf(
			"The following extensions are duplicated: %s",
			strings.Join(duplicateOIDsList, ", ")),
	}
}
