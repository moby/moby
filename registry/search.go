package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/errdefs"

	"github.com/docker/distribution/registry/client/auth"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var acceptedSearchFilterTags = map[string]bool{
	"is-automated": true,
	"is-official":  true,
	"stars":        true,
}

// Search queries the public registry for repositories matching the specified
// search term and filters.
func (s *defaultService) Search(ctx context.Context, searchFilters filters.Args, term string, limit int, authConfig *registry.AuthConfig, headers map[string][]string) ([]registry.SearchResult, error) {
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

	unfilteredResult, err := s.searchUnfiltered(ctx, term, limit, authConfig, dockerversion.DockerUserAgent(ctx), headers)
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

	return filteredResults, nil
}

func (s *defaultService) searchUnfiltered(ctx context.Context, term string, limit int, authConfig *registry.AuthConfig, userAgent string, headers map[string][]string) (*registry.SearchResults, error) {
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

	endpoint, err := newV1Endpoint(index, userAgent, headers)
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

		modifiers := Headers(userAgent, nil)
		v2Client, err := v2AuthHTTPClient(endpoint.URL, endpoint.client.Transport, modifiers, creds, scopes)
		if err != nil {
			return nil, err
		}
		// Copy non transport http client features
		v2Client.Timeout = endpoint.client.Timeout
		v2Client.CheckRedirect = endpoint.client.CheckRedirect
		v2Client.Jar = endpoint.client.Jar

		logrus.Debugf("using v2 client for search to %s", endpoint.URL)
		client = v2Client
	} else {
		client = endpoint.client
		if err := authorizeClient(client, authConfig, endpoint); err != nil {
			return nil, err
		}
	}

	return newSession(client, endpoint).searchRepositories(remoteName, limit)
}
