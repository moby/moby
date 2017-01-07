package client

import (
	"net/url"
	"regexp"

	distreference "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/filters"
	"github.com/pkg/errors"
)

var headerRegexp = regexp.MustCompile(`\ADocker/.+\s\((.+)\)\z`)

// getDockerOS returns the operating system based on the server header from the daemon.
func getDockerOS(serverHeader string) string {
	var osType string
	matches := headerRegexp.FindStringSubmatch(serverHeader)
	if len(matches) > 0 {
		osType = matches[1]
	}
	return osType
}

// getFiltersQuery returns a url query with "filters" query term, based on the
// filters provided.
func getFiltersQuery(f filters.Args) (url.Values, error) {
	query := url.Values{}
	if f.Len() > 0 {
		filterJSON, err := filters.ToParam(f)
		if err != nil {
			return query, err
		}
		query.Set("filters", filterJSON)
	}
	return query, nil
}

func parseNamed(name string) (distreference.Named, error) {
	named, err := distreference.ParseNamed(name)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing reference: %q is not a valid repository/tag", name)
	}

	return named, nil
}
