package registry

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/api/types/registry"
	"github.com/sirupsen/logrus"
)

// SearchServiceOptions holds configuration options for the search service,
// such as mirrors and insecure registries. It currently wraps ServiceOptions,
// but does not use the ServiceOptions.AllowNondistributableArtifacts field.
type SearchServiceOptions struct {
	ServiceOptions
}

// NewSearchService returns a new instance of SearchService ready to be
// installed into an engine.
func NewSearchService(options SearchServiceOptions) (*SearchService, error) {
	config, err := newServiceConfig(options.ServiceOptions)
	if err != nil {
		return nil, err
	}
	return &SearchService{config: config}, err
}

// SearchService is a service to search a registry. It tracks configuration data
// such as a list of mirrors.
type SearchService struct {
	config *serviceConfig
	mu     sync.RWMutex
}

// Search queries the public registry for images matching the specified
// search terms, and returns the results.
func (s *SearchService) Search(_ context.Context, term string, limit int, authConfig *registry.AuthConfig, userAgent string, headers map[string][]string) (*registry.SearchResults, error) {
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
