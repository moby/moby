// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package common

import (
	"fmt"
	"strings"

	"github.com/spdx/tools-golang/json/marshal"
)

// Creator is a wrapper around the Creator SPDX field. The SPDX field contains two values, which requires special
// handling in order to marshal/unmarshal it to/from Go data types.
type Creator struct {
	Creator string
	// CreatorType should be one of "Person", "Organization", or "Tool"
	CreatorType string
}

// UnmarshalJSON takes an annotator in the typical one-line format and parses it into a Creator struct.
// This function is also used when unmarshalling YAML
func (c *Creator) UnmarshalJSON(data []byte) error {
	str := string(data)
	str = strings.Trim(str, "\"")
	fields := strings.SplitN(str, ":", 2)

	if len(fields) != 2 {
		return fmt.Errorf("failed to parse Creator '%s'", str)
	}

	c.CreatorType = strings.TrimSpace(fields[0])
	c.Creator = strings.TrimSpace(fields[1])

	return nil
}

// MarshalJSON converts the receiver into a slice of bytes representing a Creator in string form.
// This function is also used with marshalling to YAML
func (c Creator) MarshalJSON() ([]byte, error) {
	if c.Creator != "" {
		return marshal.JSON(fmt.Sprintf("%s: %s", c.CreatorType, c.Creator))
	}

	return []byte{}, nil
}
