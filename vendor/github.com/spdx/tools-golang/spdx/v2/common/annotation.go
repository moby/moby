// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package common

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Annotator struct {
	Annotator string
	// including AnnotatorType: one of "Person", "Organization" or "Tool"
	AnnotatorType string
}

// UnmarshalJSON takes an annotator in the typical one-line format and parses it into an Annotator struct.
// This function is also used when unmarshalling YAML
func (a *Annotator) UnmarshalJSON(data []byte) error {
	// annotator will simply be a string
	annotatorStr := string(data)
	annotatorStr = strings.Trim(annotatorStr, "\"")

	annotatorFields := strings.SplitN(annotatorStr, ": ", 2)

	if len(annotatorFields) != 2 {
		return fmt.Errorf("failed to parse Annotator '%s'", annotatorStr)
	}

	a.AnnotatorType = annotatorFields[0]
	a.Annotator = annotatorFields[1]

	return nil
}

// MarshalJSON converts the receiver into a slice of bytes representing an Annotator in string form.
// This function is also used when marshalling to YAML
func (a Annotator) MarshalJSON() ([]byte, error) {
	if a.Annotator != "" {
		return json.Marshal(fmt.Sprintf("%s: %s", a.AnnotatorType, a.Annotator))
	}

	return []byte{}, nil
}
