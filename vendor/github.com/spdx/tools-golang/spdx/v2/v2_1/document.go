// Package spdx contains the struct definition for an SPDX Document
// and its constituent parts.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package v2_1

import (
	converter "github.com/anchore/go-struct-converter"

	"github.com/spdx/tools-golang/spdx/v2/common"
)

const Version = "SPDX-2.1"
const DataLicense = "CC0-1.0"

// ExternalDocumentRef is a reference to an external SPDX document
// as defined in section 2.6 for version 2.1 of the spec.
type ExternalDocumentRef struct {
	// DocumentRefID is the ID string defined in the start of the
	// reference. It should _not_ contain the "DocumentRef-" part
	// of the mandatory ID string.
	DocumentRefID common.DocumentID `json:"externalDocumentId"`

	// URI is the URI defined for the external document
	URI string `json:"spdxDocument"`

	// Checksum is the actual hash data
	Checksum common.Checksum `json:"checksum"`
}

// Document is an SPDX Document for version 2.1 of the spec.
// See https://spdx.org/sites/cpstandard/files/pages/files/spdxversion2.1.pdf
type Document struct {
	// 2.1: SPDX Version; should be in the format "SPDX-2.1"
	// Cardinality: mandatory, one
	SPDXVersion string `json:"spdxVersion"`

	// 2.2: Data License; should be "CC0-1.0"
	// Cardinality: mandatory, one
	DataLicense string `json:"dataLicense"`

	// 2.3: SPDX Identifier; should be "DOCUMENT" to represent
	//      mandatory identifier of SPDXRef-DOCUMENT
	// Cardinality: mandatory, one
	SPDXIdentifier common.ElementID `json:"SPDXID"`

	// 2.4: Document Name
	// Cardinality: mandatory, one
	DocumentName string `json:"name"`

	// 2.5: Document Namespace
	// Cardinality: mandatory, one
	DocumentNamespace string `json:"documentNamespace"`

	// 2.6: External Document References
	// Cardinality: optional, one or many
	ExternalDocumentReferences []ExternalDocumentRef `json:"externalDocumentRefs,omitempty"`

	// 2.11: Document Comment
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
	Reviews []*Review `json:"-"`
}

func (d *Document) ConvertFrom(_ interface{}) error {
	d.SPDXVersion = Version
	return nil
}

var _ converter.ConvertFrom = (*Document)(nil)
