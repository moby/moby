package tags

import (
	"fmt"
	"regexp"

	"github.com/docker/distribution/reference"
)

// DefaultTag defines the default tag used when performing images related actions and no tag string is specified
const DefaultTag = "latest"

var anchoredTagRegexp = regexp.MustCompile(`^` + reference.TagRegexp.String() + `$`)

// ErrTagInvalidFormat is returned if tag is invalid.
type ErrTagInvalidFormat struct {
	name string
}

func (e ErrTagInvalidFormat) Error() string {
	return fmt.Sprintf("Illegal tag name (%s): only [A-Za-z0-9_.-] are allowed ('.' and '-' are NOT allowed in the initial), minimum 1, maximum 128 in length", e.name)
}

// ValidateTagName validates the name of a tag.
// It returns an error if the given name is an emtpy string.
// If name is not valid, it returns ErrTagInvalidFormat
func ValidateTagName(name string) error {
	if name == "" {
		return fmt.Errorf("tag name can't be empty")
	}

	if !anchoredTagRegexp.MatchString(name) {
		return ErrTagInvalidFormat{name}
	}
	return nil
}
