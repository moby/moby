// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package v2_2

import "github.com/spdx/tools-golang/spdx/common"

// Relationship is a Relationship section of an SPDX Document for
// version 2.2 of the spec.
type Relationship struct {

	// 11.1: Relationship
	// Cardinality: optional, one or more; one per Relationship
	//              one mandatory for SPDX Document with multiple packages
	// RefA and RefB are first and second item
	// Relationship is type from 11.1.1
	RefA         common.DocElementID `json:"spdxElementId"`
	RefB         common.DocElementID `json:"relatedSpdxElement"`
	Relationship string              `json:"relationshipType"`

	// 11.2: Relationship Comment
	// Cardinality: optional, one
	RelationshipComment string `json:"comment,omitempty"`
}
