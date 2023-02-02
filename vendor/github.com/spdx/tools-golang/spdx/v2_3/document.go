// Package spdx contains the struct definition for an SPDX Document
// and its constituent parts.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package v2_3

import "github.com/spdx/tools-golang/spdx/common"

// ExternalDocumentRef is a reference to an external SPDX document
// as defined in section 6.6 for version 2.3 of the spec.
type ExternalDocumentRef struct {
	// DocumentRefID is the ID string defined in the start of the
	// reference. It should _not_ contain the "DocumentRef-" part
	// of the mandatory ID string.
	DocumentRefID string `json:"externalDocumentId"`

	// URI is the URI defined for the external document
	URI string `json:"spdxDocument"`

	// Checksum is the actual hash data
	Checksum common.Checksum `json:"checksum"`
}

// Document is an SPDX Document for version 2.3 of the spec.
// See https://spdx.github.io/spdx-spec/v2.3/document-creation-information
type Document struct {
	// 6.1: SPDX Version; should be in the format "SPDX-2.3"
	// Cardinality: mandatory, one
	SPDXVersion string `json:"spdxVersion"`

	// 6.2: Data License; should be "CC0-1.0"
	// Cardinality: mandatory, one
	DataLicense string `json:"dataLicense"`

	// 6.3: SPDX Identifier; should be "DOCUMENT" to represent
	//      mandatory identifier of SPDXRef-DOCUMENT
	// Cardinality: mandatory, one
	SPDXIdentifier common.ElementID `json:"SPDXID"`

	// 6.4: Document Name
	// Cardinality: mandatory, one
	DocumentName string `json:"name"`

	// 6.5: Document Namespace
	// Cardinality: mandatory, one
	DocumentNamespace string `json:"documentNamespace"`

	// 6.6: External Document References
	// Cardinality: optional, one or many
	ExternalDocumentReferences []ExternalDocumentRef `json:"externalDocumentRefs,omitempty"`

	// 6.11: Document Comment
	// Cardinality: optional, one
	DocumentComment string `json:"comment,omitempty"`

	CreationInfo  *CreationInfo   `json:"creationInfo"`
	Packages      []*Package      `json:"packages,omitempty"`
	Files         []*File         `json:"files,omitempty"`
	OtherLicenses []*OtherLicense `json:"hasExtractedLicensingInfos,omitempty"`
	Relationships []*Relationship `json:"relationships,omitempty"`
	Annotations   []*Annotation   `json:"annotations,omitempty"`
	Snippets      []Snippet       `json:"snippets,omitempty"`

	// DEPRECATED in version 2.0 of spec
	Reviews []*Review `json:"-" yaml:"-"`
}
