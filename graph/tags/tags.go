package tags

import (
	"github.com/docker/distribution/registry/api/v2"
	derr "github.com/docker/docker/errors"
)

// DefaultTag defines the default tag used when performing images related actions and no tag string is specified
const DefaultTag = "latest"

// ValidateTagName validates the name of a tag.
// It returns an error if the given name is an emtpy string.
// If name does not match v2.TagNameAnchoredRegexp regexp, it returns ErrTagInvalidFormat
func ValidateTagName(name string) error {
	if name == "" {
		return derr.ErrorCodeTagNameIsEmpty
	}

	if !v2.TagNameAnchoredRegexp.MatchString(name) {
		return derr.ErrorCodeTagNameFormat.WithArgs(name)
	}
	return nil
}
