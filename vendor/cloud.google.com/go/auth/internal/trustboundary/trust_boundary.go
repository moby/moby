// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trustboundary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/internal"
	"cloud.google.com/go/auth/internal/retry"
	"cloud.google.com/go/auth/internal/transport/headers"
	"github.com/googleapis/gax-go/v2/internallog"
)

const (
	// serviceAccountAllowedLocationsEndpoint is the URL for fetching allowed locations for a given service account email.
	serviceAccountAllowedLocationsEndpoint = "https://iamcredentials.%s/v1/projects/-/serviceAccounts/%s/allowedLocations"
)

// isEnabled wraps isTrustBoundaryEnabled with sync.OnceValues to ensure it's
// called only once.
var isEnabled = sync.OnceValues(isTrustBoundaryEnabled)

// IsEnabled returns if the trust boundary feature is enabled and an error if
// the configuration is invalid. The underlying check is performed only once.
func IsEnabled() (bool, error) {
	return isEnabled()
}

// isTrustBoundaryEnabled checks if the trust boundary feature is enabled via
// GOOGLE_AUTH_TRUST_BOUNDARY_ENABLED environment variable.
//
// If the environment variable is not set, it is considered false.
//
// The environment variable is interpreted as a boolean with the following
// (case-insensitive) rules:
//   - "true", "1" are considered true.
//   - "false", "0" are considered false.
//
// Any other values will return an error.
func isTrustBoundaryEnabled() (bool, error) {
	const envVar = "GOOGLE_AUTH_TRUST_BOUNDARY_ENABLED"
	val, ok := os.LookupEnv(envVar)
	if !ok {
		return false, nil
	}
	val = strings.ToLower(val)
	switch val {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf(`invalid value for %s: %q. Must be one of "true", "false", "1", or "0"`, envVar, val)
	}
}

// ConfigProvider provides specific configuration for trust boundary lookups.
type ConfigProvider interface {
	// GetTrustBoundaryEndpoint returns the endpoint URL for the trust boundary lookup.
	GetTrustBoundaryEndpoint(ctx context.Context) (url string, err error)
	// GetUniverseDomain returns the universe domain associated with the credential.
	// It may return an error if the universe domain cannot be determined.
	GetUniverseDomain(ctx context.Context) (string, error)
}

// AllowedLocationsResponse is the structure of the response from the Trust Boundary API.
type AllowedLocationsResponse struct {
	// Locations is the list of allowed locations.
	Locations []string `json:"locations"`
	// EncodedLocations is the encoded representation of the allowed locations.
	EncodedLocations string `json:"encodedLocations"`
}

// fetchTrustBoundaryData fetches the trust boundary data from the API.
func fetchTrustBoundaryData(ctx context.Context, client *http.Client, url string, token *auth.Token, logger *slog.Logger) (*internal.TrustBoundaryData, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if client == nil {
		return nil, errors.New("trustboundary: HTTP client is required")
	}

	if url == "" {
		return nil, errors.New("trustboundary: URL cannot be empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("trustboundary: failed to create trust boundary request: %w", err)
	}

	if token == nil || token.Value == "" {
		return nil, errors.New("trustboundary: access token required for lookup API authentication")
	}
	headers.SetAuthHeader(token, req)
	logger.DebugContext(ctx, "trust boundary request", "request", internallog.HTTPRequest(req, nil))

	retryer := retry.New()
	var response *http.Response
	for {
		response, err = client.Do(req)

		var statusCode int
		if response != nil {
			statusCode = response.StatusCode
		}
		pause, shouldRetry := retryer.Retry(statusCode, err)

		if !shouldRetry {
			break
		}

		if response != nil {
			// Drain and close the body to reuse the connection
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
		}

		if err := retry.Sleep(ctx, pause); err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, fmt.Errorf("trustboundary: failed to fetch trust boundary: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("trustboundary: failed to read trust boundary response: %w", err)
	}

	logger.DebugContext(ctx, "trust boundary response", "response", internallog.HTTPResponse(response, body))

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("trustboundary: trust boundary request failed with status: %s, body: %s", response.Status, string(body))
	}

	apiResponse := AllowedLocationsResponse{}
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("trustboundary: failed to unmarshal trust boundary response: %w", err)
	}

	if apiResponse.EncodedLocations == "" {
		return nil, errors.New("trustboundary: invalid API response: encodedLocations is empty")
	}

	return internal.NewTrustBoundaryData(apiResponse.Locations, apiResponse.EncodedLocations), nil
}

// serviceAccountConfig holds configuration for SA trust boundary lookups.
// It implements the ConfigProvider interface.
type serviceAccountConfig struct {
	ServiceAccountEmail string
	UniverseDomain      string
}

// NewServiceAccountConfigProvider creates a new config for service accounts.
func NewServiceAccountConfigProvider(saEmail, universeDomain string) ConfigProvider {
	return &serviceAccountConfig{
		ServiceAccountEmail: saEmail,
		UniverseDomain:      universeDomain,
	}
}

// GetTrustBoundaryEndpoint returns the formatted URL for fetching allowed locations
// for the configured service account and universe domain.
func (sac *serviceAccountConfig) GetTrustBoundaryEndpoint(ctx context.Context) (url string, err error) {
	if sac.ServiceAccountEmail == "" {
		return "", errors.New("trustboundary: service account email cannot be empty for config")
	}
	ud := sac.UniverseDomain
	if ud == "" {
		ud = internal.DefaultUniverseDomain
	}
	return fmt.Sprintf(serviceAccountAllowedLocationsEndpoint, ud, sac.ServiceAccountEmail), nil
}

// GetUniverseDomain returns the configured universe domain, defaulting to
// [internal.DefaultUniverseDomain] if not explicitly set.
func (sac *serviceAccountConfig) GetUniverseDomain(ctx context.Context) (string, error) {
	if sac.UniverseDomain == "" {
		return internal.DefaultUniverseDomain, nil
	}
	return sac.UniverseDomain, nil
}

// DataProvider fetches and caches trust boundary Data.
// It implements the DataProvider interface and uses a ConfigProvider
// to get type-specific details for the lookup.
type DataProvider struct {
	client         *http.Client
	configProvider ConfigProvider
	data           *internal.TrustBoundaryData
	logger         *slog.Logger
	base           auth.TokenProvider
}

// NewProvider wraps the provided base [auth.TokenProvider] to create a new
// provider that injects tokens with trust boundary data. It uses the provided
// HTTP client and configProvider to fetch the data and attach it to the token's
// metadata.
func NewProvider(client *http.Client, configProvider ConfigProvider, logger *slog.Logger, base auth.TokenProvider) (*DataProvider, error) {
	if client == nil {
		return nil, errors.New("trustboundary: HTTP client cannot be nil for DataProvider")
	}
	if configProvider == nil {
		return nil, errors.New("trustboundary: ConfigProvider cannot be nil for DataProvider")
	}
	p := &DataProvider{
		client:         client,
		configProvider: configProvider,
		logger:         internallog.New(logger),
		base:           base,
	}
	return p, nil
}

// Token retrieves a token from the base provider and injects it with trust
// boundary data.
func (p *DataProvider) Token(ctx context.Context) (*auth.Token, error) {
	// Get the original token.
	token, err := p.base.Token(ctx)
	if err != nil {
		return nil, err
	}

	tbData, err := p.GetTrustBoundaryData(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("trustboundary: error fetching the trust boundary data: %w", err)
	}
	if tbData != nil {
		if token.Metadata == nil {
			token.Metadata = make(map[string]interface{})
		}
		token.Metadata[internal.TrustBoundaryDataKey] = *tbData
	}
	return token, nil
}

// GetTrustBoundaryData retrieves the trust boundary data.
// It first checks the universe domain: if it's non-default, a NoOp is returned.
// Otherwise, it checks a local cache. If the data is not cached as NoOp,
// it fetches new data from the endpoint provided by its ConfigProvider,
// using the given accessToken for authentication. Results are cached.
// If fetching fails, it returns previously cached data if available, otherwise the fetch error.
func (p *DataProvider) GetTrustBoundaryData(ctx context.Context, token *auth.Token) (*internal.TrustBoundaryData, error) {
	// Check the universe domain.
	uniDomain, err := p.configProvider.GetUniverseDomain(ctx)
	if err != nil {
		return nil, fmt.Errorf("trustboundary: error getting universe domain: %w", err)
	}
	if uniDomain != "" && uniDomain != internal.DefaultUniverseDomain {
		if p.data == nil || p.data.EncodedLocations != internal.TrustBoundaryNoOp {
			p.data = internal.NewNoOpTrustBoundaryData()
		}
		return p.data, nil
	}

	// Check cache for a no-op result from a previous API call.
	cachedData := p.data
	if cachedData != nil && cachedData.EncodedLocations == internal.TrustBoundaryNoOp {
		return cachedData, nil
	}

	// Get the endpoint
	url, err := p.configProvider.GetTrustBoundaryEndpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("trustboundary: error getting the lookup endpoint: %w", err)
	}

	// Proceed to fetch new data.
	newData, fetchErr := fetchTrustBoundaryData(ctx, p.client, url, token, p.logger)

	if fetchErr != nil {
		// Fetch failed. Fallback to cachedData if available.
		if cachedData != nil {
			return cachedData, nil // Successful fallback
		}
		// No cache to fallback to.
		return nil, fmt.Errorf("trustboundary: failed to fetch trust boundary data for endpoint %s and no cache available: %w", url, fetchErr)
	}

	// Fetch successful. Update cache.
	p.data = newData
	return newData, nil
}

// GCEConfigProvider implements ConfigProvider for GCE environments.
// It lazily fetches and caches the necessary metadata (service account email, universe domain)
// from the GCE metadata server.
type GCEConfigProvider struct {
	// universeDomainProvider provides the universe domain and underlying metadata client.
	universeDomainProvider *internal.ComputeUniverseDomainProvider

	// Caching for service account email
	saOnce     sync.Once
	saEmail    string
	saEmailErr error

	// Caching for universe domain
	udOnce sync.Once
	ud     string
	udErr  error
}

// NewGCEConfigProvider creates a new GCEConfigProvider
// which uses the provided gceUDP to interact with the GCE metadata server.
func NewGCEConfigProvider(gceUDP *internal.ComputeUniverseDomainProvider) *GCEConfigProvider {
	// The validity of gceUDP and its internal MetadataClient will be checked
	// within the GetTrustBoundaryEndpoint and GetUniverseDomain methods.
	return &GCEConfigProvider{
		universeDomainProvider: gceUDP,
	}
}

func (g *GCEConfigProvider) fetchSA(ctx context.Context) {
	if g.universeDomainProvider == nil || g.universeDomainProvider.MetadataClient == nil {
		g.saEmailErr = errors.New("trustboundary: GCEConfigProvider not properly initialized (missing ComputeUniverseDomainProvider or MetadataClient)")
		return
	}
	mdClient := g.universeDomainProvider.MetadataClient
	saEmail, err := mdClient.EmailWithContext(ctx, "default")
	if err != nil {
		g.saEmailErr = fmt.Errorf("trustboundary: GCE config: failed to get service account email: %w", err)
		return
	}
	g.saEmail = saEmail
}

func (g *GCEConfigProvider) fetchUD(ctx context.Context) {
	if g.universeDomainProvider == nil || g.universeDomainProvider.MetadataClient == nil {
		g.udErr = errors.New("trustboundary: GCEConfigProvider not properly initialized (missing ComputeUniverseDomainProvider or MetadataClient)")
		return
	}
	ud, err := g.universeDomainProvider.GetProperty(ctx)
	if err != nil {
		g.udErr = fmt.Errorf("trustboundary: GCE config: failed to get universe domain: %w", err)
		return
	}
	if ud == "" {
		ud = internal.DefaultUniverseDomain
	}
	g.ud = ud
}

// GetTrustBoundaryEndpoint constructs the trust boundary lookup URL for a GCE environment.
// It uses cached metadata (service account email, universe domain) after the first call.
func (g *GCEConfigProvider) GetTrustBoundaryEndpoint(ctx context.Context) (string, error) {
	g.saOnce.Do(func() { g.fetchSA(ctx) })
	if g.saEmailErr != nil {
		return "", g.saEmailErr
	}
	g.udOnce.Do(func() { g.fetchUD(ctx) })
	if g.udErr != nil {
		return "", g.udErr
	}
	return fmt.Sprintf(serviceAccountAllowedLocationsEndpoint, g.ud, g.saEmail), nil
}

// GetUniverseDomain retrieves the universe domain from the GCE metadata server.
// It uses a cached value after the first call.
func (g *GCEConfigProvider) GetUniverseDomain(ctx context.Context) (string, error) {
	g.udOnce.Do(func() { g.fetchUD(ctx) })
	if g.udErr != nil {
		return "", g.udErr
	}
	return g.ud, nil
}
