package httputils // import "github.com/docker/docker/api/server/httputils"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// BoolValue transforms a form value in different formats into a boolean type.
func BoolValue(r *http.Request, k string) bool {
	switch strings.ToLower(strings.TrimSpace(r.FormValue(k))) {
	case "", "0", "no", "false", "none":
		return false
	default:
		return true
	}
}

// BoolValueOrDefault returns the default bool passed if the query param is
// missing, otherwise it's just a proxy to boolValue above.
func BoolValueOrDefault(r *http.Request, k string, d bool) bool {
	if _, ok := r.Form[k]; !ok {
		return d
	}
	return BoolValue(r, k)
}

// Uint32Value parses a form value into an uint32 type. It returns an error
// if the field is not set, empty, incorrectly formatted, or out of range.
func Uint32Value(r *http.Request, field string) (uint32, error) {
	// strconv.ParseUint returns an "strconv.ErrSyntax" for negative values,
	// not an "out of range". Strip the prefix before parsing, and use it
	// later to detect valid, but negative values.
	v, isNeg := strings.CutPrefix(r.Form.Get(field), "-")
	if v == "" || v[0] == '+' {
		// Fast-path for invalid values.
		return 0, strconv.ErrSyntax
	}

	i, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		// Unwrap to remove the 'strconv.ParseUint: parsing "some-invalid-value":' prefix.
		return 0, errors.Unwrap(err)
	}
	if isNeg {
		return 0, strconv.ErrRange
	}
	return uint32(i), nil
}

// Int64ValueOrZero parses a form value into an int64 type.
// It returns 0 if the parsing fails.
func Int64ValueOrZero(r *http.Request, k string) int64 {
	val, err := Int64ValueOrDefault(r, k, 0)
	if err != nil {
		return 0
	}
	return val
}

// Int64ValueOrDefault parses a form value into an int64 type. If there is an
// error, returns the error. If there is no value returns the default value.
func Int64ValueOrDefault(r *http.Request, field string, def int64) (int64, error) {
	if r.Form.Get(field) != "" {
		value, err := strconv.ParseInt(r.Form.Get(field), 10, 64)
		return value, err
	}
	return def, nil
}

// RepoTagReference parses form values "repo" and "tag" and returns a valid
// reference with repository and tag.
// If repo is empty, then a nil reference is returned.
// If no tag is given, then the default "latest" tag is set.
func RepoTagReference(repo, tag string) (reference.NamedTagged, error) {
	if repo == "" {
		return nil, nil
	}

	ref, err := reference.ParseNormalizedNamed(repo)
	if err != nil {
		return nil, err
	}

	if _, isDigested := ref.(reference.Digested); isDigested {
		return nil, fmt.Errorf("cannot import digest reference")
	}

	if tag != "" {
		return reference.WithTag(ref, tag)
	}

	withDefaultTag := reference.TagNameOnly(ref)

	namedTagged, ok := withDefaultTag.(reference.NamedTagged)
	if !ok {
		return nil, fmt.Errorf("unexpected reference: %q", ref.String())
	}

	return namedTagged, nil
}

// ArchiveOptions stores archive information for different operations.
type ArchiveOptions struct {
	Name string
	Path string
}

type badParameterError struct {
	param string
}

func (e badParameterError) Error() string {
	return "bad parameter: " + e.param + "cannot be empty"
}

func (e badParameterError) InvalidParameter() {}

// ArchiveFormValues parses form values and turns them into ArchiveOptions.
// It fails if the archive name and path are not in the request.
func ArchiveFormValues(r *http.Request, vars map[string]string) (ArchiveOptions, error) {
	if err := ParseForm(r); err != nil {
		return ArchiveOptions{}, err
	}

	name := vars["name"]
	if name == "" {
		return ArchiveOptions{}, badParameterError{"name"}
	}
	path := r.Form.Get("path")
	if path == "" {
		return ArchiveOptions{}, badParameterError{"path"}
	}
	return ArchiveOptions{name, path}, nil
}

// DecodePlatform decodes the OCI platform JSON string into a Platform struct.
func DecodePlatform(platformJSON string) (*ocispec.Platform, error) {
	var p ocispec.Platform

	if err := json.Unmarshal([]byte(platformJSON), &p); err != nil {
		return nil, errdefs.InvalidParameter(errors.Wrap(err, "failed to parse platform"))
	}

	hasAnyOptional := (p.Variant != "" || p.OSVersion != "" || len(p.OSFeatures) > 0)

	if p.OS == "" && p.Architecture == "" && hasAnyOptional {
		return nil, errdefs.InvalidParameter(errors.New("optional platform fields provided, but OS and Architecture are missing"))
	}

	if p.OS == "" || p.Architecture == "" {
		return nil, errdefs.InvalidParameter(errors.New("both OS and Architecture must be provided"))
	}

	return &p, nil
}

// DecodePlatforms decodes the OCI platform JSON string into a Platform struct.
//
// Typically, the argument is a value of: r.Form["platform"]
func DecodePlatforms(platformJSONs []string) ([]ocispec.Platform, error) {
	if len(platformJSONs) == 0 {
		return nil, nil
	}

	var output []ocispec.Platform
	for _, platform := range platformJSONs {
		p, err := DecodePlatform(platform)
		if err != nil {
			return nil, err
		}
		output = append(output, *p)
	}
	return output, nil
}
