package client

import (
	"context"
	"encoding/json"
	"net/url"
	"path"
	"sort"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/versions"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerCreate creates a new container based on the given configuration.
// It can be associated with a name, but it's not mandatory.
func (cli *Client) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	if config == nil {
		return container.CreateResponse{}, cerrdefs.ErrInvalidArgument.WithMessage("config is nil")
	}

	var response container.CreateResponse

	if hostConfig != nil {
		hostConfig.CapAdd = normalizeCapabilities(hostConfig.CapAdd)
		hostConfig.CapDrop = normalizeCapabilities(hostConfig.CapDrop)
	}

	// FIXME(thaJeztah): remove this once we updated our (integration) tests;
	//  some integration tests depend on this to test old API versions; see https://github.com/moby/moby/pull/51120#issuecomment-3376224865
	if config.MacAddress != "" { //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.44.
		// Make sure we negotiated (if the client is configured to do so),
		// as code below contains API-version specific handling of options.
		//
		// Normally, version-negotiation (if enabled) would not happen until
		// the API request is made.
		if err := cli.checkVersion(ctx); err != nil {
			return response, err
		}
		if versions.GreaterThanOrEqualTo(cli.ClientVersion(), "1.44") {
			// Since API 1.44, the container-wide MacAddress is deprecated and triggers a WARNING if it's specified.
			//
			// FIXME(thaJeztah): remove the field from the API
			config.MacAddress = "" //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.44.
		}
	}

	query := url.Values{}
	if platform != nil {
		if p := formatPlatform(*platform); p != "unknown" {
			query.Set("platform", p)
		}
	}

	if containerName != "" {
		query.Set("name", containerName)
	}

	body := container.CreateRequest{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}

	resp, err := cli.post(ctx, "/containers/create", query, body, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	return response, err
}

// formatPlatform returns a formatted string representing platform (e.g., "linux/arm/v7").
//
// It is a fork of [platforms.Format], and does not yet support "os.version",
// as [[platforms.FormatAll] does.
//
// [platforms.Format]: https://github.com/containerd/platforms/blob/v1.0.0-rc.1/platforms.go#L309-L316
// [platforms.FormatAll]: https://github.com/containerd/platforms/blob/v1.0.0-rc.1/platforms.go#L318-L330
func formatPlatform(platform ocispec.Platform) string {
	if platform.OS == "" {
		return "unknown"
	}
	return path.Join(platform.OS, platform.Architecture, platform.Variant)
}

// hasEndpointSpecificMacAddress checks whether one of the endpoint in networkingConfig has a MacAddress defined.
func hasEndpointSpecificMacAddress(networkingConfig *network.NetworkingConfig) bool {
	if networkingConfig == nil {
		return false
	}
	for _, endpoint := range networkingConfig.EndpointsConfig {
		if endpoint.MacAddress != "" {
			return true
		}
	}
	return false
}

// allCapabilities is a magic value for "all capabilities"
const allCapabilities = "ALL"

// normalizeCapabilities normalizes capabilities to their canonical form,
// removes duplicates, and sorts the results.
//
// It is similar to [caps.NormalizeLegacyCapabilities],
// but performs no validation based on supported capabilities.
//
// [caps.NormalizeLegacyCapabilities]: https://github.com/moby/moby/blob/v28.3.2/oci/caps/utils.go#L56
func normalizeCapabilities(caps []string) []string {
	var normalized []string

	unique := make(map[string]struct{})
	for _, c := range caps {
		c = normalizeCap(c)
		if _, ok := unique[c]; ok {
			continue
		}
		unique[c] = struct{}{}
		normalized = append(normalized, c)
	}

	sort.Strings(normalized)
	return normalized
}

// normalizeCap normalizes a capability to its canonical format by upper-casing
// and adding a "CAP_" prefix (if not yet present). It also accepts the "ALL"
// magic-value.
func normalizeCap(capability string) string {
	capability = strings.ToUpper(capability)
	if capability == allCapabilities {
		return capability
	}
	if !strings.HasPrefix(capability, "CAP_") {
		capability = "CAP_" + capability
	}
	return capability
}
