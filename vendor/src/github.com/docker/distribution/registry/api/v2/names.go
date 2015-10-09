package v2

import (
	"fmt"
	"regexp"
	"strings"
)

// TODO(stevvooe): Move these definitions to the future "reference" package.
// While they are used with v2 definitions, their relevance expands beyond.

const (
	// RepositoryNameTotalLengthMax is the maximum total number of characters in
	// a repository name
	RepositoryNameTotalLengthMax = 255
)

// domainLabelRegexp represents the following RFC-2396 BNF construct:
//   domainlabel   = alphanum | alphanum *( alphanum | "-" ) alphanum
var domainLabelRegexp = regexp.MustCompile(`[a-z0-9](?:-*[a-z0-9])*`)

// RepositoryNameComponentRegexp restricts registry path component names to
// the allow valid hostnames according to: https://www.ietf.org/rfc/rfc2396.txt
// with the following differences:
//  1) It DOES NOT allow for fully-qualified domain names, which include a
//    trailing '.', e.g. "google.com."
//  2) It DOES NOT restrict 'top-level' domain labels to start with just alpha
//    characters.
//  3) It DOES allow for underscores to appear in the same situations as dots.
//
// RFC-2396 uses the BNF construct:
//   hostname      = *( domainlabel "." ) toplabel [ "." ]
var RepositoryNameComponentRegexp = regexp.MustCompile(
	domainLabelRegexp.String() + `(?:[._]` + domainLabelRegexp.String() + `)*`)

// RepositoryNameComponentAnchoredRegexp is the version of
// RepositoryNameComponentRegexp which must completely match the content
var RepositoryNameComponentAnchoredRegexp = regexp.MustCompile(`^` + RepositoryNameComponentRegexp.String() + `$`)

// RepositoryNameRegexp builds on RepositoryNameComponentRegexp to allow
// multiple path components, separated by a forward slash.
var RepositoryNameRegexp = regexp.MustCompile(`(?:` + RepositoryNameComponentRegexp.String() + `/)*` + RepositoryNameComponentRegexp.String())

// TagNameRegexp matches valid tag names. From docker/docker:graph/tags.go.
var TagNameRegexp = regexp.MustCompile(`[\w][\w.-]{0,127}`)

// TagNameAnchoredRegexp matches valid tag names, anchored at the start and
// end of the matched string.
var TagNameAnchoredRegexp = regexp.MustCompile("^" + TagNameRegexp.String() + "$")

var (
	// ErrRepositoryNameEmpty is returned for empty, invalid repository names.
	ErrRepositoryNameEmpty = fmt.Errorf("repository name must have at least one component")

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
	if name == "" {
		return ErrRepositoryNameEmpty
	}

	if len(name) > RepositoryNameTotalLengthMax {
		return ErrRepositoryNameLong
	}

	components := strings.Split(name, "/")

	for _, component := range components {
		if !RepositoryNameComponentAnchoredRegexp.MatchString(component) {
			return ErrRepositoryNameComponentInvalid
		}
	}

	return nil
}
