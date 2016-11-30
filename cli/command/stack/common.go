package stack

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/composetransform"
)

func getStackFilter(namespace string) filters.Args {
	filter := filters.NewArgs()
	filter.Add("label", composetransform.LabelNamespace+"="+namespace)
	return filter
}

func getServices(
	ctx context.Context,
	apiclient client.APIClient,
	namespace string,
) ([]swarm.Service, error) {
	return apiclient.ServiceList(
		ctx,
		types.ServiceListOptions{Filters: getStackFilter(namespace)})
}

func getStackNetworks(
	ctx context.Context,
	apiclient client.APIClient,
	namespace string,
) ([]types.NetworkResource, error) {
	return apiclient.NetworkList(
		ctx,
		types.NetworkListOptions{Filters: getStackFilter(namespace)})
}
