// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package v2_1

// OtherLicense is an Other License Information section of an
// SPDX Document for version 2.1 of the spec.
type OtherLicense struct {
	// 6.1: License Identifier: "LicenseRef-[idstring]"
	// Cardinality: conditional (mandatory, one) if license is not
	//              on SPDX License List
	LicenseIdentifier string `json:"licenseId"`

	// 6.2: Extracted Text
	// Cardinality: conditional (mandatory, one) if there is a
	//              License Identifier assigned
	ExtractedText string `json:"extractedText"`

	// 6.3: License Name: single line of text or "NOASSERTION"
	// Cardinality: conditional (mandatory, one) if license is not
	//              on SPDX License List
	LicenseName string `json:"name,omitempty"`

	// 6.4: License Cross Reference
	// Cardinality: conditional (optional, one or many) if license
	//              is not on SPDX License List
	LicenseCrossReferences []string `json:"seeAlsos,omitempty"`

	// 6.5: License Comment
	// Cardinality: optional, one
	LicenseComment string `json:"comment,omitempty"`
}
