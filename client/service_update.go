package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
)

// ServiceUpdateOptions contains the options to be used for updating services.
type ServiceUpdateOptions struct {
	// EncodedRegistryAuth is the encoded registry authorization credentials to
	// use when updating the service.
	//
	// This field follows the format of the X-Registry-Auth header.
	EncodedRegistryAuth string

	// TODO(stevvooe): Consider moving the version parameter of ServiceUpdate
	// into this field. While it does open API users up to racy writes, most
	// users may not need that level of consistency in practice.

	// RegistryAuthFrom specifies where to find the registry authorization
	// credentials if they are not given in EncodedRegistryAuth. Valid
	// values are "spec" and "previous-spec".
	RegistryAuthFrom string

	// Rollback indicates whether a server-side rollback should be
	// performed. When this is set, the provided spec will be ignored.
	// The valid values are "previous" and "none". An empty value is the
	// same as "none".
	Rollback string

	// QueryRegistry indicates whether the service update requires
	// contacting a registry. A registry may be contacted to retrieve
	// the image digest and manifest, which in turn can be used to update
	// platform or other information about the service.
	QueryRegistry bool
}

// ServiceUpdateResult represents the result of a service update.
type ServiceUpdateResult struct {
	// Warnings contains any warnings that occurred during the update.
	Warnings []string
}

// ServiceUpdate updates a Service. The version number is required to avoid
// conflicting writes. It must be the value as set *before* the update.
// You can find this value in the [swarm.Service.Meta] field, which can
// be found using [Client.ServiceInspectWithRaw].
func (cli *Client) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options ServiceUpdateOptions) (ServiceUpdateResult, error) {
	serviceID, err := trimID("service", serviceID)
	if err != nil {
		return ServiceUpdateResult{}, err
	}

	if err := validateServiceSpec(service); err != nil {
		return ServiceUpdateResult{}, err
	}

	query := url.Values{}
	if options.RegistryAuthFrom != "" {
		query.Set("registryAuthFrom", options.RegistryAuthFrom)
	}

	if options.Rollback != "" {
		query.Set("rollback", options.Rollback)
	}

	query.Set("version", version.String())

	// ensure that the image is tagged
	var warnings []string
	switch {
	case service.TaskTemplate.ContainerSpec != nil:
		if taggedImg := imageWithTagString(service.TaskTemplate.ContainerSpec.Image); taggedImg != "" {
			service.TaskTemplate.ContainerSpec.Image = taggedImg
		}
		if options.QueryRegistry {
			resolveWarning := resolveContainerSpecImage(ctx, cli, &service.TaskTemplate, options.EncodedRegistryAuth)
			warnings = append(warnings, resolveWarning)
		}
	case service.TaskTemplate.PluginSpec != nil:
		if taggedImg := imageWithTagString(service.TaskTemplate.PluginSpec.Remote); taggedImg != "" {
			service.TaskTemplate.PluginSpec.Remote = taggedImg
		}
		if options.QueryRegistry {
			resolveWarning := resolvePluginSpecRemote(ctx, cli, &service.TaskTemplate, options.EncodedRegistryAuth)
			warnings = append(warnings, resolveWarning)
		}
	}

	headers := http.Header{}
	if options.EncodedRegistryAuth != "" {
		headers.Set(registry.AuthHeader, options.EncodedRegistryAuth)
	}
	resp, err := cli.post(ctx, "/services/"+serviceID+"/update", query, service, headers)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ServiceUpdateResult{}, err
	}

	var response swarm.ServiceUpdateResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	warnings = append(warnings, response.Warnings...)
	return ServiceUpdateResult{Warnings: warnings}, err
}
