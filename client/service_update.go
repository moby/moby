package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/versions"
)

// ServiceUpdate updates a Service. The version number is required to avoid conflicting writes.
// It should be the value as set *before* the update. You can find this value in the Meta field
// of swarm.Service, which can be found using ServiceInspectWithRaw.
func (cli *Client) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (types.ServiceUpdateResponse, error) {
	var (
		query    = url.Values{}
		response = types.ServiceUpdateResponse{}
	)

	if options.RegistryAuthFrom != "" {
		query.Set("registryAuthFrom", options.RegistryAuthFrom)
	}

	if options.Rollback != "" {
		query.Set("rollback", options.Rollback)
	}

	query.Set("version", version.String())

	if err := validateServiceSpec(service); err != nil {
		return response, err
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

	headers := map[string][]string{}
	if versions.LessThan(cli.version, "1.30") {
		// the custom "version" header was used by engine API before 20.10
		// (API 1.30) to switch between client- and server-side lookup of
		// image digests.
		headers["version"] = []string{cli.version}
	}
	if options.EncodedRegistryAuth != "" {
		headers[registry.AuthHeader] = []string{options.EncodedRegistryAuth}
	}
	resp, err := cli.post(ctx, "/services/"+serviceID+"/update", query, service, headers)
	defer ensureReaderClosed(resp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)
	if resolveWarning != "" {
		response.Warnings = append(response.Warnings, resolveWarning)
	}

	return response, err
}
