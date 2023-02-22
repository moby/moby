package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"strconv"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

var acceptedSearchFilterTags = map[string]bool{
	"is-automated": true,
	"is-official":  true,
	"stars":        true,
}

// SearchRegistryForImages queries the registry for images matching
// term. authConfig is used to login.
//
// TODO: this could be implemented in a registry service instead of the image
// service.
func (i *ImageService) SearchRegistryForImages(ctx context.Context, searchFilters filters.Args, term string, limit int,
	authConfig *registry.AuthConfig,
	headers map[string][]string) (*registry.SearchResults, error) {
	if err := searchFilters.Validate(acceptedSearchFilterTags); err != nil {
		return nil, err
	}

	isAutomated, err := searchFilters.GetBoolOrDefault("is-automated", false)
	if err != nil {
		return nil, err
	}
	isOfficial, err := searchFilters.GetBoolOrDefault("is-official", false)
	if err != nil {
		return nil, err
	}

	hasStarFilter := 0
	if searchFilters.Contains("stars") {
		hasStars := searchFilters.Get("stars")
		for _, hasStar := range hasStars {
			iHasStar, err := strconv.Atoi(hasStar)
			if err != nil {
				return nil, errdefs.InvalidParameter(errors.Wrapf(err, "invalid filter 'stars=%s'", hasStar))
			}
			if iHasStar > hasStarFilter {
				hasStarFilter = iHasStar
			}
		}
	}

	unfilteredResult, err := i.registryService.Search(ctx, term, limit, authConfig, dockerversion.DockerUserAgent(ctx), headers)
	if err != nil {
		return nil, err
	}

	filteredResults := []registry.SearchResult{}
	for _, result := range unfilteredResult.Results {
		if searchFilters.Contains("is-automated") {
			if isAutomated != result.IsAutomated {
				continue
			}
		}
		if searchFilters.Contains("is-official") {
			if isOfficial != result.IsOfficial {
				continue
			}
		}
		if searchFilters.Contains("stars") {
			if result.StarCount < hasStarFilter {
				continue
			}
		}
		filteredResults = append(filteredResults, result)
	}

	return &registry.SearchResults{
		Query:      unfilteredResult.Query,
		NumResults: len(filteredResults),
		Results:    filteredResults,
	}, nil
}
