package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/registry"
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
				return nil, invalidParameterErr{errors.Wrapf(err, "invalid filter 'stars=%s'", hasStar)}
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
	if strings.Contains(term, "://") {
		return nil, invalidParamf("invalid repository name: repository name (%s) should not have a scheme", term)
	}

	indexName, remoteName := splitReposSearchTerm(term)

	// Search is a long-running operation, just lock s.config to avoid block others.
	s.mu.RLock()
	index := newIndexInfo(s.config, indexName)
	s.mu.RUnlock()
	if index.Official {
		// If pull "library/foo", it's stored locally under "foo"
		remoteName = strings.TrimPrefix(remoteName, "library/")
	}

	endpoint, err := newV1Endpoint(ctx, index, headers)
	if err != nil {
		return nil, err
	}

	var client *http.Client
	if authConfig != nil && authConfig.IdentityToken != "" && authConfig.Username != "" {
		creds := NewStaticCredentialStore(authConfig)

		// TODO(thaJeztah); is there a reason not to include other headers here? (originally added in 19d48f0b8ba59eea9f2cac4ad1c7977712a6b7ac)
		modifiers := Headers(headers.Get("User-Agent"), nil)
		v2Client, err := v2AuthHTTPClient(endpoint.URL, endpoint.client.Transport, modifiers, creds, []auth.Scope{
			auth.RegistryScope{Name: "catalog", Actions: []string{"search"}},
		})
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
		if err := authorizeClient(ctx, client, authConfig, endpoint); err != nil {
			return nil, err
		}
	}

	return searchRepositories(ctx, client, endpoint, remoteName, limit)
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

// newIndexInfo returns IndexInfo configuration from indexName
func newIndexInfo(config *serviceConfig, indexName string) *registry.IndexInfo {
	indexName = normalizeIndexName(indexName)

	// Return any configured index info, first.
	if index, ok := config.IndexConfigs[indexName]; ok {
		return index
	}

	// Construct a non-configured index info.
	return &registry.IndexInfo{
		Name:    indexName,
		Mirrors: []string{},
		Secure:  config.isSecureIndex(indexName),
	}
}

// defaultSearchLimit is the default value for maximum number of returned search results.
const defaultSearchLimit = 25

// searchRepositories performs a search against the remote repository
func searchRepositories(ctx context.Context, client *http.Client, ep *v1Endpoint, term string, limit int) (*registry.SearchResults, error) {
	if limit == 0 {
		limit = defaultSearchLimit
	}
	if limit < 1 || limit > 100 {
		return nil, invalidParamf("limit %d is outside the range of [1, 100]", limit)
	}
	u := ep.String() + "search?q=" + url.QueryEscape(term) + "&n=" + url.QueryEscape(strconv.Itoa(limit))
	log.G(ctx).WithField("url", u).Debug("searchRepositories")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, invalidParamWrapf(err, "error building request")
	}
	// Have the AuthTransport send authentication, when logged in.
	req.Header.Set("X-Docker-Token", "true")
	res, err := client.Do(req)
	if err != nil {
		return nil, systemErr{err}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		// TODO(thaJeztah): return upstream response body for errors (see https://github.com/moby/moby/issues/27286).
		// TODO(thaJeztah): handle other status-codes to return correct error-type
		return nil, errUnknown{fmt.Errorf("unexpected status code %d", res.StatusCode)}
	}
	result := &registry.SearchResults{}
	err = json.NewDecoder(res.Body).Decode(result)
	if err != nil {
		return nil, systemErr{errors.Wrap(err, "error decoding registry search results")}
	}
	return result, nil
}
