// +build experimental

package stack

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

const (
	labelNamespace = "com.docker.stack.namespace"
)

func getStackLabels(namespace string, labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[labelNamespace] = namespace
	return labels
}

func getStackFilter(namespace string) filters.Args {
	filter := filters.NewArgs()
	filter.Add("label", labelNamespace+"="+namespace)
	return filter
}

func getServices(
	ctx context.Context,
	apiclient client.APIClient,
	namespace string,
) ([]swarm.Service, error) {
	return apiclient.ServiceList(
		ctx,
		types.ServiceListOptions{Filter: getStackFilter(namespace)})
}

func getNetworks(
	ctx context.Context,
	apiclient client.APIClient,
	namespace string,
) ([]types.NetworkResource, error) {
	return apiclient.NetworkList(
		ctx,
		types.NetworkListOptions{Filters: getStackFilter(namespace)})
}
