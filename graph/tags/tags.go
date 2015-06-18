package tags

import (
	"fmt"
	"regexp"
)

const DEFAULTTAG = "latest"

var (
	//FIXME this regex also exists in registry/v2/regexp.go
	validTagName = regexp.MustCompile(`^[\w][\w.-]{0,127}$`)
)

// ValidateTagName validates the name of a tag
func ValidateTagName(name string) error {
	if name == "" {
		return fmt.Errorf("tag name can't be empty")
	}
	if !validTagName.MatchString(name) {
		return fmt.Errorf("Illegal tag name (%s): only [A-Za-z0-9_.-] are allowed, minimum 1, maximum 128 in length", name)
	}
	return nil
}
