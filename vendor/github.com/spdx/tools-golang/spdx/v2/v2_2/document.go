// Package spdx contains the struct definition for an SPDX Document
// and its constituent parts.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package v2_2

import (
	"encoding/json"
	"fmt"

	converter "github.com/anchore/go-struct-converter"

	"github.com/spdx/tools-golang/spdx/v2/common"
)

const Version = "SPDX-2.2"
const DataLicense = "CC0-1.0"

// ExternalDocumentRef is a reference to an external SPDX document
// as defined in section 6.6 for version 2.2 of the spec.
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

// Document is an SPDX Document for version 2.2 of the spec.
// See https://spdx.github.io/spdx-spec/v2-draft/ (DRAFT)
type Document struct {
	// 6.1: SPDX Version; should be in the format "SPDX-2.2"
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
	Reviews []*Review `json:"-"`
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
		refA := r.RefA
		refB := r.RefB
		rel := r.Relationship

		// we need to serialize the opposite for CONTAINED_BY and DESCRIBED_BY
		// so that it will match when we try to de-duplicate during deserialization.
		switch r.Relationship {
		case common.TypeRelationshipContainedBy:
			rel = common.TypeRelationshipContains
			refA = r.RefB
			refB = r.RefA
		case common.TypeRelationshipDescribeBy:
			rel = common.TypeRelationshipDescribe
			refA = r.RefB
			refB = r.RefA
		}
		return fmt.Sprintf("%v-%v->%v", common.RenderDocElementID(refA), rel, common.RenderDocElementID(refB))
	}

	// remove null relationships
	for i := 0; i < len(d.Relationships); i++ {
		if d.Relationships[i] == nil {
			d.Relationships = append(d.Relationships[0:i], d.Relationships[i+1:]...)
			i--
		}
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
