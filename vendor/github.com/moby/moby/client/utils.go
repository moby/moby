package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/filters"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type emptyIDError string

func (e emptyIDError) InvalidParameter() {}

func (e emptyIDError) Error() string {
	return "invalid " + string(e) + " name or ID: value is empty"
}

// trimID trims the given object-ID / name, returning an error if it's empty.
func trimID(objType, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", emptyIDError(objType)
	}
	return id, nil
}

// getFiltersQuery returns a url query with "filters" query term, based on the
// filters provided.
func getFiltersQuery(f filters.Args) (url.Values, error) {
	query := url.Values{}
	if f.Len() > 0 {
		filterJSON, err := filters.ToJSON(f)
		if err != nil {
			return query, err
		}
		query.Set("filters", filterJSON)
	}
	return query, nil
}

// encodePlatforms marshals the given platform(s) to JSON format, to
// be used for query-parameters for filtering / selecting platforms.
func encodePlatforms(platform ...ocispec.Platform) ([]string, error) {
	if len(platform) == 0 {
		return []string{}, nil
	}
	if len(platform) == 1 {
		p, err := encodePlatform(&platform[0])
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	}

	seen := make(map[string]struct{}, len(platform))
	out := make([]string, 0, len(platform))
	for i := range platform {
		p, err := encodePlatform(&platform[i])
		if err != nil {
			return nil, err
		}
		if _, ok := seen[p]; !ok {
			out = append(out, p)
			seen[p] = struct{}{}
		}
	}
	return out, nil
}

// encodePlatform marshals the given platform to JSON format, to
// be used for query-parameters for filtering / selecting platforms. It
// is used as a helper for encodePlatforms,
func encodePlatform(platform *ocispec.Platform) (string, error) {
	p, err := json.Marshal(platform)
	if err != nil {
		return "", fmt.Errorf("%w: invalid platform: %v", cerrdefs.ErrInvalidArgument, err)
	}
	return string(p), nil
}

// encodeLegacyFilters encodes Args in the legacy format as used in API v1.21 and older.
// where values are a list of strings, instead of a set.
//
// Don't use in any new code; use [filters.ToJSON]] instead.
func encodeLegacyFilters(currentFormat string) (string, error) {
	// The Args.fields field is not exported, but used to marshal JSON,
	// so we'll marshal to the new format, then unmarshal to get the
	// fields, and marshal again.
	//
	// This is far from optimal, but this code is only used for deprecated
	// API versions, so should not be hit commonly.
	var argsFields map[string]map[string]bool
	err := json.Unmarshal([]byte(currentFormat), &argsFields)
	if err != nil {
		return "", err
	}

	buf, err := json.Marshal(convertArgsToSlice(argsFields))
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func convertArgsToSlice(f map[string]map[string]bool) map[string][]string {
	m := map[string][]string{}
	for k, v := range f {
		values := []string{}
		for kk := range v {
			if v[kk] {
				values = append(values, kk)
			}
		}
		m[k] = values
	}
	return m
}
