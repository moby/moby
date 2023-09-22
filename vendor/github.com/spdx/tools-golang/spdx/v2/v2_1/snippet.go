// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package v2_1

import (
	"github.com/spdx/tools-golang/spdx/v2/common"
)

// Snippet is a Snippet section of an SPDX Document for version 2.1 of the spec.
type Snippet struct {

	// 5.1: Snippet SPDX Identifier: "SPDXRef-[idstring]"
	// Cardinality: mandatory, one
	SnippetSPDXIdentifier common.ElementID `json:"SPDXID"`

	// 5.2: Snippet from File SPDX Identifier
	// Cardinality: mandatory, one
	SnippetFromFileSPDXIdentifier common.ElementID `json:"snippetFromFile"`

	// Ranges denotes the start/end byte offsets or line numbers that the snippet is relevant to
	Ranges []common.SnippetRange `json:"ranges"`

	// 5.5: Snippet Concluded License: SPDX License Expression, "NONE" or "NOASSERTION"
	// Cardinality: mandatory, one
	SnippetLicenseConcluded string `json:"licenseConcluded"`

	// 5.6: License Information in Snippet: SPDX License Expression, "NONE" or "NOASSERTION"
	// Cardinality: optional, one or many
	LicenseInfoInSnippet []string `json:"licenseInfoInSnippets,omitempty"`

	// 5.7: Snippet Comments on License
	// Cardinality: optional, one
	SnippetLicenseComments string `json:"licenseComments,omitempty"`

	// 5.8: Snippet Copyright Text: copyright notice(s) text, "NONE" or "NOASSERTION"
	// Cardinality: mandatory, one
	SnippetCopyrightText string `json:"copyrightText"`

	// 5.9: Snippet Comment
	// Cardinality: optional, one
	SnippetComment string `json:"comment,omitempty"`

	// 5.10: Snippet Name
	// Cardinality: optional, one
	SnippetName string `json:"name,omitempty"`
}
