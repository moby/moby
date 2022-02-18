package images // import "github.com/moby/moby/daemon/images"

import (
	"context"
	"strconv"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/filters"
	registrytypes "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/dockerversion"
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
func (i *ImageService) SearchRegistryForImages(ctx context.Context, filtersArgs string, term string, limit int,
	authConfig *types.AuthConfig,
	headers map[string][]string) (*registrytypes.SearchResults, error) {

	searchFilters, err := filters.FromJSON(filtersArgs)
	if err != nil {
		return nil, err
	}
	if err := searchFilters.Validate(acceptedSearchFilterTags); err != nil {
		return nil, err
	}

	var isAutomated, isOfficial bool
	var hasStarFilter = 0
	if searchFilters.Contains("is-automated") {
		if searchFilters.UniqueExactMatch("is-automated", "true") {
			isAutomated = true
		} else if !searchFilters.UniqueExactMatch("is-automated", "false") {
			return nil, invalidFilter{"is-automated", searchFilters.Get("is-automated")}
		}
	}
	if searchFilters.Contains("is-official") {
		if searchFilters.UniqueExactMatch("is-official", "true") {
			isOfficial = true
		} else if !searchFilters.UniqueExactMatch("is-official", "false") {
			return nil, invalidFilter{"is-official", searchFilters.Get("is-official")}
		}
	}
	if searchFilters.Contains("stars") {
		hasStars := searchFilters.Get("stars")
		for _, hasStar := range hasStars {
			iHasStar, err := strconv.Atoi(hasStar)
			if err != nil {
				return nil, invalidFilter{"stars", hasStar}
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

	filteredResults := []registrytypes.SearchResult{}
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

	return &registrytypes.SearchResults{
		Query:      unfilteredResult.Query,
		NumResults: len(filteredResults),
		Results:    filteredResults,
	}, nil
}
