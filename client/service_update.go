package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/versions"
)

// ServiceUpdate updates a Service. The version number is required to avoid
// conflicting writes. It must be the value as set *before* the update.
// You can find this value in the [swarm.Service.Meta] field, which can
// be found using [Client.ServiceInspectWithRaw].
func (cli *Client) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options ServiceUpdateOptions) (swarm.ServiceUpdateResponse, error) {
	serviceID, err := trimID("service", serviceID)
	if err != nil {
		return swarm.ServiceUpdateResponse{}, err
	}

	// Make sure we negotiated (if the client is configured to do so),
	// as code below contains API-version specific handling of options.
	//
	// Normally, version-negotiation (if enabled) would not happen until
	// the API request is made.
	if err := cli.checkVersion(ctx); err != nil {
		return swarm.ServiceUpdateResponse{}, err
	}

	query := url.Values{}
	if options.RegistryAuthFrom != "" {
		query.Set("registryAuthFrom", options.RegistryAuthFrom)
	}

	if options.Rollback != "" {
		query.Set("rollback", options.Rollback)
	}

	query.Set("version", version.String())

	if err := validateServiceSpec(service); err != nil {
		return swarm.ServiceUpdateResponse{}, err
	}

	// ensure that the image is tagged
	var resolveWarning string
	switch {
	case service.TaskTemplate.ContainerSpec != nil:
		if taggedImg := imageWithTagString(service.TaskTemplate.ContainerSpec.Image); taggedImg != "" {
			service.TaskTemplate.ContainerSpec.Image = taggedImg
		}
		if options.QueryRegistry {
			resolveWarning = resolveContainerSpecImage(ctx, cli, &service.TaskTemplate, options.EncodedRegistryAuth)
		}
	case service.TaskTemplate.PluginSpec != nil:
		if taggedImg := imageWithTagString(service.TaskTemplate.PluginSpec.Remote); taggedImg != "" {
			service.TaskTemplate.PluginSpec.Remote = taggedImg
		}
		if options.QueryRegistry {
			resolveWarning = resolvePluginSpecRemote(ctx, cli, &service.TaskTemplate, options.EncodedRegistryAuth)
		}
	}

	headers := http.Header{}
	if versions.LessThan(cli.version, "1.30") {
		// the custom "version" header was used by engine API before 20.10
		// (API 1.30) to switch between client- and server-side lookup of
		// image digests.
		headers["version"] = []string{cli.version}
	}
	if options.EncodedRegistryAuth != "" {
		headers.Set(registry.AuthHeader, options.EncodedRegistryAuth)
	}
	resp, err := cli.post(ctx, "/services/"+serviceID+"/update", query, service, headers)
	defer ensureReaderClosed(resp)
	if err != nil {
		return swarm.ServiceUpdateResponse{}, err
	}

	var response swarm.ServiceUpdateResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if resolveWarning != "" {
		response.Warnings = append(response.Warnings, resolveWarning)
	}

	return response, err
}
