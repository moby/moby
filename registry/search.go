package registry

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

var acceptedSearchFilterTags = map[string]bool{
	"is-automated": true, // Deprecated: the "is_automated" field is deprecated and will always be false in the future.
	"is-official":  true,
	"stars":        true,
}

// Search queries the public registry for repositories matching the specified
// search term and filters.
func (s *Service) Search(ctx context.Context, searchFilters filters.Args, term string, limit int, authConfig *registry.AuthConfig, headers map[string][]string) ([]registry.SearchResult, error) {
	if err := searchFilters.Validate(acceptedSearchFilterTags); err != nil {
		return nil, err
	}

	isAutomated, err := searchFilters.GetBoolOrDefault("is-automated", false)
	if err != nil {
		return nil, err
	}

	// "is-automated" is deprecated and filtering for `true` will yield no results.
	if isAutomated {
		return []registry.SearchResult{}, nil
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

	unfilteredResult, err := s.searchUnfiltered(ctx, term, limit, authConfig, headers)
	if err != nil {
		return nil, err
	}

	filteredResults := []registry.SearchResult{}
	for _, result := range unfilteredResult.Results {
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
		// "is-automated" is deprecated and the value in Docker Hub search
		// results is untrustworthy. Force it to false so as to not mislead our
		// clients.
		result.IsAutomated = false //nolint:staticcheck  // ignore SA1019 (field is deprecated)
		filteredResults = append(filteredResults, result)
	}

	return filteredResults, nil
}

func (s *Service) searchUnfiltered(ctx context.Context, term string, limit int, authConfig *registry.AuthConfig, headers http.Header) (*registry.SearchResults, error) {
	// TODO Use ctx when searching for repositories
	if hasScheme(term) {
		return nil, invalidParamf("invalid repository name: repository name (%s) should not have a scheme", term)
	}

	indexName, remoteName := splitReposSearchTerm(term)

	// Search is a long-running operation, just lock s.config to avoid block others.
	s.mu.RLock()
	index, err := newIndexInfo(s.config, indexName)
	s.mu.RUnlock()

	if err != nil {
		return nil, err
	}
	if index.Official {
		// If pull "library/foo", it's stored locally under "foo"
		remoteName = strings.TrimPrefix(remoteName, "library/")
	}

	endpoint, err := newV1Endpoint(index, headers)
	if err != nil {
		return nil, err
	}

	var client *http.Client
	if authConfig != nil && authConfig.IdentityToken != "" && authConfig.Username != "" {
		creds := NewStaticCredentialStore(authConfig)
		scopes := []auth.Scope{
			auth.RegistryScope{
				Name:    "catalog",
				Actions: []string{"search"},
			},
		}

		// TODO(thaJeztah); is there a reason not to include other headers here? (originally added in 19d48f0b8ba59eea9f2cac4ad1c7977712a6b7ac)
		modifiers := Headers(headers.Get("User-Agent"), nil)
		v2Client, err := v2AuthHTTPClient(endpoint.URL, endpoint.client.Transport, modifiers, creds, scopes)
		if err != nil {
			return nil, err
		}
		// Copy non transport http client features
		v2Client.Timeout = endpoint.client.Timeout
		v2Client.CheckRedirect = endpoint.client.CheckRedirect
		v2Client.Jar = endpoint.client.Jar

		log.G(ctx).Debugf("using v2 client for search to %s", endpoint.URL)
		client = v2Client
	} else {
		client = endpoint.client
		if err := authorizeClient(client, authConfig, endpoint); err != nil {
			return nil, err
		}
	}

	return newSession(client, endpoint).searchRepositories(remoteName, limit)
}

// splitReposSearchTerm breaks a search term into an index name and remote name
func splitReposSearchTerm(reposName string) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Hub repository (ex: samalba/hipache or ubuntu),
		// use the default Docker Hub registry (docker.io)
		return IndexName, reposName
	}
	return nameParts[0], nameParts[1]
}

// ParseSearchIndexInfo will use repository name to get back an indexInfo.
//
// TODO(thaJeztah) this function is only used by the CLI, and used to get
// information of the registry (to provide credentials if needed). We should
// move this function (or equivalent) to the CLI, as it's doing too much just
// for that.
func ParseSearchIndexInfo(reposName string) (*registry.IndexInfo, error) {
	indexName, _ := splitReposSearchTerm(reposName)
	return newIndexInfo(emptyServiceConfig, indexName)
}
