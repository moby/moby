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
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerCreate creates a new container based on the given configuration.
// It can be associated with a name, but it's not mandatory.
func (cli *Client) ContainerCreate(ctx context.Context, options ContainerCreateOptions) (ContainerCreateResult, error) {
	cfg := options.Config

	if cfg == nil {
		cfg = &container.Config{}
	}

	if options.Image != "" {
		if cfg.Image != "" {
			return ContainerCreateResult{}, cerrdefs.ErrInvalidArgument.WithMessage("either Image or config.Image should be set")
		}
		newCfg := *cfg
		newCfg.Image = options.Image
		cfg = &newCfg
	}

	if cfg.Image == "" {
		return ContainerCreateResult{}, cerrdefs.ErrInvalidArgument.WithMessage("config.Image or Image is required")
	}

	var response container.CreateResponse

	if options.HostConfig != nil {
		options.HostConfig.CapAdd = normalizeCapabilities(options.HostConfig.CapAdd)
		options.HostConfig.CapDrop = normalizeCapabilities(options.HostConfig.CapDrop)
	}

	query := url.Values{}
	if options.Platform != nil {
		if p := formatPlatform(*options.Platform); p != "unknown" {
			query.Set("platform", p)
		}
	}

	if options.Name != "" {
		query.Set("name", options.Name)
	}

	body := container.CreateRequest{
		Config:           cfg,
		HostConfig:       options.HostConfig,
		NetworkingConfig: options.NetworkingConfig,
	}

	resp, err := cli.post(ctx, "/containers/create", query, body, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerCreateResult{}, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	return ContainerCreateResult{ID: response.ID, Warnings: response.Warnings}, err
}

// formatPlatform returns a formatted string representing platform (e.g., "linux/arm/v7").
//
// It is a fork of [platforms.Format], and does not yet support "os.version",
// as [platforms.FormatAll] does.
//
// [platforms.Format]: https://github.com/containerd/platforms/blob/v1.0.0-rc.1/platforms.go#L309-L316
// [platforms.FormatAll]: https://github.com/containerd/platforms/blob/v1.0.0-rc.1/platforms.go#L318-L330
func formatPlatform(platform ocispec.Platform) string {
	if platform.OS == "" {
		return "unknown"
	}
	return path.Join(platform.OS, platform.Architecture, platform.Variant)
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
