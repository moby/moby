// Package v2_3 Package contains the struct definition for an SPDX Document
// and its constituent parts.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package v2_3

import (
	"encoding/json"
	"fmt"

	converter "github.com/anchore/go-struct-converter"

	"github.com/spdx/tools-golang/spdx/v2/common"
)

const Version = "SPDX-2.3"
const DataLicense = "CC0-1.0"

// ExternalDocumentRef is a reference to an external SPDX document as defined in section 6.6
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

// Document is an SPDX Document:
// See https://spdx.github.io/spdx-spec/v2.3/document-creation-information
type Document struct {
	// 6.1: SPDX Version; should be in the format "SPDX-<version>"
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

func (d *Document) ConvertFrom(_ interface{}) error {
	d.SPDXVersion = Version
	return nil
}

var _ converter.ConvertFrom = (*Document)(nil)

func (d *Document) UnmarshalJSON(b []byte) error {
	type doc Document
	type extras struct {
		DocumentDescribes []common.DocElementID `json:"documentDescribes"`
	}

	var d2 doc
	if err := json.Unmarshal(b, &d2); err != nil {
		return err
	}

	var e extras
	if err := json.Unmarshal(b, &e); err != nil {
		return err
	}

	*d = Document(d2)

	relationshipExists := map[string]bool{}
	serializeRel := func(r *Relationship) string {
		return fmt.Sprintf("%v-%v->%v", common.RenderDocElementID(r.RefA), r.Relationship, common.RenderDocElementID(r.RefB))
	}

	// index current list of relationships to ensure no duplication
	for _, r := range d.Relationships {
		relationshipExists[serializeRel(r)] = true
	}

	// build relationships for documentDescribes field
	for _, id := range e.DocumentDescribes {
		r := &Relationship{
			RefA: common.DocElementID{
				ElementRefID: d.SPDXIdentifier,
			},
			RefB:         id,
			Relationship: common.TypeRelationshipDescribe,
		}

		if !relationshipExists[serializeRel(r)] {
			d.Relationships = append(d.Relationships, r)
			relationshipExists[serializeRel(r)] = true
		}
	}

	// build relationships for package hasFiles field
	// build relationships for package hasFiles field
	for _, p := range d.Packages {
		for _, f := range p.hasFiles {
			r := &Relationship{
				RefA: common.DocElementID{
					ElementRefID: p.PackageSPDXIdentifier,
				},
				RefB:         f,
				Relationship: common.TypeRelationshipContains,
			}
			if !relationshipExists[serializeRel(r)] {
				d.Relationships = append(d.Relationships, r)
				relationshipExists[serializeRel(r)] = true
			}
		}

		p.hasFiles = nil
	}

	return nil
}

var _ json.Unmarshaler = (*Document)(nil)
