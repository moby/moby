package names // import "github.com/docker/docker/daemon/names"

import (
	"regexp"
	"strings"

	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// RestrictedNameChars collects the characters allowed to represent a name, normally used to validate container and volume names.
const RestrictedNameChars = `[a-zA-Z0-9][a-zA-Z0-9_.-]`

// RestrictedNamePattern is a regular expression to validate names against the collection of restricted characters.
var RestrictedNamePattern = regexp.MustCompile(`^` + RestrictedNameChars + `+$`)

// ValidateContainerName is to check container's name is valid or not
func ValidateContainerName(name string) (bool, error) {
	valid := RestrictedNamePattern.MatchString(strings.TrimPrefix(name, "/"))
	if valid {
		return valid, nil
	}
	return valid, errdefs.InvalidParameter(errors.Errorf("Invalid container name (%s), only %s are allowed", name, RestrictedNameChars))
}

// ValidateName is to check container's name is valid or not
func ValidateName(name string) (bool, error) {
	if len(name) == 0 {
		return true, nil
	}
	return ValidateContainerName(name)
}
