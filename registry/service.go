package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"crypto/tls"
	"net/url"
	"strings"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/sirupsen/logrus"
)

// Service is the interface defining what a registry service should implement.
type Service interface {
	Auth(ctx context.Context, authConfig *registry.AuthConfig, userAgent string) (status, token string, err error)
	LookupPullEndpoints(hostname string) (endpoints []APIEndpoint, err error)
	LookupPushEndpoints(hostname string) (endpoints []APIEndpoint, err error)
	ResolveRepository(name reference.Named) (*RepositoryInfo, error)
	Search(ctx context.Context, term string, limit int, authConfig *registry.AuthConfig, userAgent string, headers map[string][]string) (*registry.SearchResults, error)
	ServiceConfig() *registry.ServiceConfig
	LoadAllowNondistributableArtifacts([]string) error
	LoadMirrors([]string) error
	LoadInsecureRegistries([]string) error
}

// defaultService is a registry service. It tracks configuration data such as a list
// of mirrors.
type defaultService struct {
	config *serviceConfig
	mu     sync.RWMutex
}

// NewService returns a new instance of defaultService ready to be
// installed into an engine.
func NewService(options ServiceOptions) (Service, error) {
	config, err := newServiceConfig(options)

	return &defaultService{config: config}, err
}

// ServiceConfig returns a copy of the public registry service's configuration.
func (s *defaultService) ServiceConfig() *registry.ServiceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.copy()
}

// LoadAllowNondistributableArtifacts loads allow-nondistributable-artifacts registries for Service.
func (s *defaultService) LoadAllowNondistributableArtifacts(registries []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.config.loadAllowNondistributableArtifacts(registries)
}

// LoadMirrors loads registry mirrors for Service
func (s *defaultService) LoadMirrors(mirrors []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.config.loadMirrors(mirrors)
}

// LoadInsecureRegistries loads insecure registries for Service
func (s *defaultService) LoadInsecureRegistries(registries []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.config.loadInsecureRegistries(registries)
}

// Auth contacts the public registry with the provided credentials,
// and returns OK if authentication was successful.
// It can be used to verify the validity of a client's credentials.
func (s *defaultService) Auth(ctx context.Context, authConfig *registry.AuthConfig, userAgent string) (status, token string, err error) {
	// TODO Use ctx when searching for repositories
	var registryHostName = IndexHostname

	if authConfig.ServerAddress != "" {
		serverAddress := authConfig.ServerAddress
		if !strings.HasPrefix(serverAddress, "https://") && !strings.HasPrefix(serverAddress, "http://") {
			serverAddress = "https://" + serverAddress
		}
		u, err := url.Parse(serverAddress)
		if err != nil {
			return "", "", invalidParamWrapf(err, "unable to parse server address")
		}
		registryHostName = u.Host
	}

	// Lookup endpoints for authentication using "LookupPushEndpoints", which
	// excludes mirrors to prevent sending credentials of the upstream registry
	// to a mirror.
	endpoints, err := s.LookupPushEndpoints(registryHostName)
	if err != nil {
		return "", "", invalidParam(err)
	}

	for _, endpoint := range endpoints {
		status, token, err = loginV2(authConfig, endpoint, userAgent)
		if err == nil {
			return
		}
		if errdefs.IsUnauthorized(err) {
			// Failed to authenticate; don't continue with (non-TLS) endpoints.
			return status, token, err
		}
		logrus.WithError(err).Infof("Error logging in to endpoint, trying next endpoint")
	}

	return "", "", err
}

// ResolveRepository splits a repository name into its components
// and configuration of the associated registry.
func (s *defaultService) ResolveRepository(name reference.Named) (*RepositoryInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return newRepositoryInfo(s.config, name)
}

// APIEndpoint represents a remote API endpoint
type APIEndpoint struct {
	Mirror                         bool
	URL                            *url.URL
	Version                        APIVersion
	AllowNondistributableArtifacts bool
	Official                       bool
	TrimHostname                   bool
	TLSConfig                      *tls.Config
}

// LookupPullEndpoints creates a list of v2 endpoints to try to pull from, in order of preference.
// It gives preference to mirrors over the actual registry, and HTTPS over plain HTTP.
func (s *defaultService) LookupPullEndpoints(hostname string) (endpoints []APIEndpoint, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lookupV2Endpoints(hostname)
}

// LookupPushEndpoints creates a list of v2 endpoints to try to push to, in order of preference.
// It gives preference to HTTPS over plain HTTP. Mirrors are not included.
func (s *defaultService) LookupPushEndpoints(hostname string) (endpoints []APIEndpoint, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allEndpoints, err := s.lookupV2Endpoints(hostname)
	if err == nil {
		for _, endpoint := range allEndpoints {
			if !endpoint.Mirror {
				endpoints = append(endpoints, endpoint)
			}
		}
	}
	return endpoints, err
}
