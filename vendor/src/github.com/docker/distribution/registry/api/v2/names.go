package v2

import (
	"fmt"
	"regexp"
	"strings"
)

// TODO(stevvooe): Move these definitions back to an exported package. While
// they are used with v2 definitions, their relevance expands beyond.
// "distribution/names" is a candidate package.

const (
	// RepositoryNameComponentMinLength is the minimum number of characters in a
	// single repository name slash-delimited component
	RepositoryNameComponentMinLength = 2

	// RepositoryNameMinComponents is the minimum number of slash-delimited
	// components that a repository name must have
	RepositoryNameMinComponents = 1

	// RepositoryNameTotalLengthMax is the maximum total number of characters in
	// a repository name
	RepositoryNameTotalLengthMax = 255
)

// RepositoryNameComponentRegexp restricts registry path component names to
// start with at least one letter or number, with following parts able to
// be separated by one period, dash or underscore.
var RepositoryNameComponentRegexp = regexp.MustCompile(`[a-z0-9]+(?:[._-][a-z0-9]+)*`)

// RepositoryNameComponentAnchoredRegexp is the version of
// RepositoryNameComponentRegexp which must completely match the content
var RepositoryNameComponentAnchoredRegexp = regexp.MustCompile(`^` + RepositoryNameComponentRegexp.String() + `$`)

// RepositoryNameRegexp builds on RepositoryNameComponentRegexp to allow
// multiple path components, separated by a forward slash.
var RepositoryNameRegexp = regexp.MustCompile(`(?:` + RepositoryNameComponentRegexp.String() + `/)*` + RepositoryNameComponentRegexp.String())

// TagNameRegexp matches valid tag names. From docker/docker:graph/tags.go.
var TagNameRegexp = regexp.MustCompile(`[\w][\w.-]{0,127}`)

// TODO(stevvooe): Contribute these exports back to core, so they are shared.

var (
	// ErrRepositoryNameComponentShort is returned when a repository name
	// contains a component which is shorter than
	// RepositoryNameComponentMinLength
	ErrRepositoryNameComponentShort = fmt.Errorf("repository name component must be %v or more characters", RepositoryNameComponentMinLength)

	// ErrRepositoryNameMissingComponents is returned when a repository name
	// contains fewer than RepositoryNameMinComponents components
	ErrRepositoryNameMissingComponents = fmt.Errorf("repository name must have at least %v components", RepositoryNameMinComponents)

	// ErrRepositoryNameLong is returned when a repository name is longer than
	// RepositoryNameTotalLengthMax
	ErrRepositoryNameLong = fmt.Errorf("repository name must not be more than %v characters", RepositoryNameTotalLengthMax)

	// ErrRepositoryNameComponentInvalid is returned when a repository name does
	// not match RepositoryNameComponentRegexp
	ErrRepositoryNameComponentInvalid = fmt.Errorf("repository name component must match %q", RepositoryNameComponentRegexp.String())
)

// ValidateRepositoryName ensures the repository name is valid for use in the
// registry. This function accepts a superset of what might be accepted by
// docker core or docker hub. If the name does not pass validation, an error,
// describing the conditions, is returned.
//
// Effectively, the name should comply with the following grammar:
//
// 	alpha-numeric := /[a-z0-9]+/
//	separator := /[._-]/
//	component := alpha-numeric [separator alpha-numeric]*
//	namespace := component ['/' component]*
//
// The result of the production, known as the "namespace", should be limited
// to 255 characters.
func ValidateRepositoryName(name string) error {
	if len(name) > RepositoryNameTotalLengthMax {
		return ErrRepositoryNameLong
	}

	components := strings.Split(name, "/")

	if len(components) < RepositoryNameMinComponents {
		return ErrRepositoryNameMissingComponents
	}

	for _, component := range components {
		if len(component) < RepositoryNameComponentMinLength {
			return ErrRepositoryNameComponentShort
		}

		if !RepositoryNameComponentAnchoredRegexp.MatchString(component) {
			return ErrRepositoryNameComponentInvalid
		}
	}

	return nil
}
