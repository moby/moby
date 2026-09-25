// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package common

import (
	"fmt"
	"strings"

	"github.com/spdx/tools-golang/json/marshal"
)

const (
	spdxRefPrefix     = "SPDXRef-"
	documentRefPrefix = "DocumentRef-"
)

// DocumentID represents the identifier string portion of an SPDX document
// identifier. DocumentID should be used for reference to a SPDX document.
// DocumentIDs should NOT contain the mandatory 'DocumentRef-' portion.
type DocumentID string

// MarshalJSON returns an DocumentRef- prefixed JSON string
func (d DocumentID) MarshalJSON() ([]byte, error) {
	return marshal.JSON(prefixDocumentId(d))
}

// UnmarshalJSON validates DocumentRef- prefixes and removes them when processing DocumentIDs
func (d *DocumentID) UnmarshalJSON(data []byte) error {
	// SPDX identifier will simply be a string
	idStr := string(data)
	idStr = strings.Trim(idStr, "\"")

	documentID, err := trimDocumentIdPrefix(idStr)
	if err != nil {
		return err
	}
	*d = documentID
	return nil
}

// prefixDocumentId adds the DocumentRef- prefix to an document ID if it does not have one
func prefixDocumentId(id DocumentID) string {
	val := string(id)
	if !strings.HasPrefix(val, documentRefPrefix) {
		return documentRefPrefix + val
	}
	return val
}

// trimDocumentIdPrefix removes the DocumentRef- prefix from an document ID string and
// returns an error if the prefix isn't present or if the resulting ID is empty.
func trimDocumentIdPrefix(id string) (DocumentID, error) {
	idWithoutPrefix, found := strings.CutPrefix(id, documentRefPrefix)
	if !found || idWithoutPrefix == "" {
		return DocumentID(""), fmt.Errorf("failed to parse DocumentID: %s", id)
	}
	return DocumentID(idWithoutPrefix), nil
}

// ElementID represents the identifier string portion of an SPDX element
// identifier. DocElementID should be used for any attributes which can
// contain identifiers defined in a different SPDX document.
// ElementIDs should NOT contain the mandatory 'SPDXRef-' portion.
type ElementID string

// MarshalJSON returns an SPDXRef- prefixed JSON string
func (d ElementID) MarshalJSON() ([]byte, error) {
	return marshal.JSON(prefixElementId(d))
}

// UnmarshalJSON validates SPDXRef- prefixes and removes them when processing ElementIDs
func (d *ElementID) UnmarshalJSON(data []byte) error {
	// SPDX identifier will simply be a string
	idStr := string(data)
	idStr = strings.Trim(idStr, "\"")

	elementID, err := trimElementIdPrefix(idStr)
	if err != nil {
		return err
	}
	*d = elementID
	return nil
}

// prefixElementId adds the SPDXRef- prefix to an element ID if it does not have one
func prefixElementId(id ElementID) string {
	val := string(id)
	if !strings.HasPrefix(val, spdxRefPrefix) {
		return spdxRefPrefix + val
	}
	return val
}

// trimElementIdPrefix removes the SPDXRef- prefix from an element ID string and
// returns an error if the prefix isn't present or if the resulting ID is empty.
func trimElementIdPrefix(id string) (ElementID, error) {
	idWithoutPrefix, found := strings.CutPrefix(id, spdxRefPrefix)
	if !found || idWithoutPrefix == "" {
		return ElementID(""), fmt.Errorf("failed to parse ElementID: %s", id)
	}
	return ElementID(idWithoutPrefix), nil
}

// DocElementID represents an SPDX element identifier that could be defined
// in a different SPDX document, and therefore could have a "DocumentRef-"
// portion, such as Relationships and Annotations.
// ElementID is used for attributes in which a "DocumentRef-" portion cannot
// appear, such as a Package or File definition (since it is necessarily
// being defined in the present document).
// DocumentRefID will be the empty string for elements defined in the
// present document.
// DocElementIDs should NOT contain the mandatory 'DocumentRef-' or
// 'SPDXRef-' portions.
// SpecialID is used ONLY if the DocElementID matches a defined set of
// permitted special values for a particular field, e.g. "NONE" or
// "NOASSERTION" for the right-hand side of Relationships. If SpecialID
// is set, DocumentRefID and ElementRefID should be empty (and vice versa).
type DocElementID struct {
	DocumentRefID DocumentID
	ElementRefID  ElementID
	SpecialID     string
}

// MarshalJSON converts the receiver into a slice of bytes representing a DocElementID in string form.
// This function is also used when marshalling to YAML
func (d DocElementID) MarshalJSON() ([]byte, error) {
	if d.DocumentRefID != "" && d.ElementRefID != "" {
		dIdStr := prefixDocumentId(d.DocumentRefID)
		eIdStr := prefixElementId(d.ElementRefID)
		return marshal.JSON(fmt.Sprintf("%s:%s", dIdStr, eIdStr))
	} else if d.ElementRefID != "" {
		return marshal.JSON(prefixElementId(d.ElementRefID))
	} else if d.SpecialID != "" {
		return marshal.JSON(d.SpecialID)
	}

	return []byte{}, fmt.Errorf("failed to marshal empty DocElementID")
}

// UnmarshalJSON takes a SPDX Identifier string parses it into a DocElementID struct.
// This function is also used when unmarshalling YAML
func (d *DocElementID) UnmarshalJSON(data []byte) error {
	// SPDX identifier will simply be a string
	idStr := string(data)
	idStr = strings.Trim(idStr, "\"")

	// handle special cases
	if idStr == "NONE" || idStr == "NOASSERTION" {
		d.SpecialID = idStr
		return nil
	}

	var idFields []string
	// handle DocumentRef- if present
	if strings.HasPrefix(idStr, documentRefPrefix) {
		// an SPDXRef can appear after a DocumentRef, separated by a colon
		idFields = strings.SplitN(idStr, ":", 2)

		documentRefID, err := trimDocumentIdPrefix(idFields[0])
		if err != nil {
			return err
		}
		d.DocumentRefID = documentRefID
		if len(idFields) == 2 {
			idStr = idFields[1]
		} else {
			return nil
		}
	}

	elementRefID, err := trimElementIdPrefix(idStr)
	if err != nil {
		return err
	}
	d.ElementRefID = elementRefID
	return nil
}

// TODO: add equivalents for LicenseRef- identifiers

// MakeDocElementID takes strings (without prefixes) for the DocumentRef-
// and SPDXRef- identifiers, and returns a DocElementID. An empty string
// should be used for the DocumentRef- portion if it is referring to the
// present document.
func MakeDocElementID(docRef string, eltRef string) DocElementID {
	return DocElementID{
		DocumentRefID: DocumentID(docRef),
		ElementRefID:  ElementID(eltRef),
	}
}

// MakeDocElementSpecial takes a "special" string (e.g. "NONE" or
// "NOASSERTION" for the right side of a Relationship), nd returns
// a DocElementID with it in the SpecialID field. Other fields will
// be empty.
func MakeDocElementSpecial(specialID string) DocElementID {
	return DocElementID{SpecialID: specialID}
}

// RenderElementID takes an ElementID and returns the string equivalent,
// with the SPDXRef- prefix reinserted.
func RenderElementID(eID ElementID) string {
	return spdxRefPrefix + string(eID)
}

// RenderDocElementID takes a DocElementID and returns the string equivalent,
// with the SPDXRef- prefix (and, if applicable, the DocumentRef- prefix)
// reinserted. If a SpecialID is present, it will be rendered verbatim and
// DocumentRefID and ElementRefID will be ignored.
func RenderDocElementID(deID DocElementID) string {
	if deID.SpecialID != "" {
		return deID.SpecialID
	}
	prefix := ""
	if deID.DocumentRefID != "" {
		prefix = documentRefPrefix + string(deID.DocumentRefID) + ":"
	}
	return prefix + spdxRefPrefix + string(deID.ElementRefID)
}
