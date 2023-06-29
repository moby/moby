package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"crypto/tls"
	"net/url"
	"strings"
	"sync"

	"github.com/containerd/containerd/log"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
)

// Service is a registry service. It tracks configuration data such as a list
// of mirrors.
type Service struct {
	config *serviceConfig
	mu     sync.RWMutex
}

// NewService returns a new instance of defaultService ready to be
// installed into an engine.
func NewService(options ServiceOptions) (*Service, error) {
	config, err := newServiceConfig(options)

	return &Service{config: config}, err
}

// ServiceConfig returns a copy of the public registry service's configuration.
func (s *Service) ServiceConfig() *registry.ServiceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.copy()
}

// ReplaceConfig prepares a transaction which will atomically replace the
// registry service's configuration when the returned commit function is called.
func (s *Service) ReplaceConfig(options ServiceOptions) (commit func(), err error) {
	config, err := newServiceConfig(options)
	if err != nil {
		return nil, err
	}
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.config = config
	}, nil
}

// Auth contacts the public registry with the provided credentials,
// and returns OK if authentication was successful.
// It can be used to verify the validity of a client's credentials.
func (s *Service) Auth(ctx context.Context, authConfig *registry.AuthConfig, userAgent string) (status, token string, err error) {
	// TODO Use ctx when searching for repositories
	registryHostName := IndexHostname

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
		log.G(ctx).WithError(err).Infof("Error logging in to endpoint, trying next endpoint")
	}

	return "", "", err
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

// ResolveRepository splits a repository name into its components
// and configuration of the associated registry.
func (s *Service) ResolveRepository(name reference.Named) (*RepositoryInfo, error) {
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
func (s *Service) LookupPullEndpoints(hostname string) (endpoints []APIEndpoint, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lookupV2Endpoints(hostname)
}

// LookupPushEndpoints creates a list of v2 endpoints to try to push to, in order of preference.
// It gives preference to HTTPS over plain HTTP. Mirrors are not included.
func (s *Service) LookupPushEndpoints(hostname string) (endpoints []APIEndpoint, err error) {
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

// IsInsecureRegistry returns true if the registry at given host is configured as
// insecure registry.
func (s *Service) IsInsecureRegistry(host string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.config.isSecureIndex(host)
}
