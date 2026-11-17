package serde

import (
	"strings"
)

// Tag represents the `document` struct field tag and associated options
type Tag struct {
	Name      string
	Ignore    bool
	OmitEmpty bool
}

// ParseTag splits a struct field tag into its name and
// comma-separated options.
func ParseTag(tagStr string) (tag Tag) {
	parts := strings.Split(tagStr, ",")
	if len(parts) == 0 {
		return tag
	}

	if name := parts[0]; name == "-" {
		tag.Name = ""
		tag.Ignore = true
	} else {
		tag.Name = name
		tag.Ignore = false
	}

	for _, opt := range parts[1:] {
		switch opt {
		case "omitempty":
			tag.OmitEmpty = true
		}
	}

	return tag
}
