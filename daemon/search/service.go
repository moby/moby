package search // import "github.com/docker/docker/daemon/search"

import (
	"context"
	"strconv"

	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/dockerversion"
	registrypkg "github.com/docker/docker/registry"
)

// registrySearch provides functions to search a registry, using the registry V1 search API.
type registrySearch interface {
	Search(ctx context.Context, term string, limit int, authConfig *registry.AuthConfig, userAgent string, header map[string][]string) (*registry.SearchResults, error)
}

var acceptedSearchFilterTags = map[string]bool{
	"is-automated": true,
	"is-official":  true,
	"stars":        true,
}

// Service provides the backend to search registries for images.
type Service struct {
	registrySearch registrySearch
}

// NewService initializes a new Service  to search registries for images.
func NewService(opts registrypkg.SearchServiceOptions) (*Service, error) {
	registrySearch, err := registrypkg.NewSearchService(opts)
	if err != nil {
		return nil, err
	}

	return &Service{registrySearch: registrySearch}, nil
}

// SearchImages queries the registry for images matching the given term and
// options.
func (i *Service) SearchImages(ctx context.Context, term string, opts registry.SearchOpts) (*registry.SearchResults, error) {
	searchFilters := opts.Filters

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
		for _, hasStar := range searchFilters.Get("stars") {
			iHasStar, err := strconv.Atoi(hasStar)
			if err != nil {
				return nil, invalidFilter{"stars", hasStar}
			}
			if iHasStar > hasStarFilter {
				hasStarFilter = iHasStar
			}
		}
	}

	unfilteredResult, err := i.registrySearch.Search(ctx, term, opts.Limit, opts.AuthConfig, dockerversion.DockerUserAgent(ctx), opts.Headers)
	if err != nil {
		return nil, err
	}

	filteredResults := make([]registry.SearchResult, 0, len(unfilteredResult.Results))
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
