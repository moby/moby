package stack

import (
	"bytes"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/compose/convert"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/testutil/assert"
	"golang.org/x/net/context"
)

type fakeClient struct {
	client.Client
	serviceList []string
	removedIDs  []string
}

func (cli *fakeClient) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
	services := []swarm.Service{}
	for _, name := range cli.serviceList {
		services = append(services, swarm.Service{
			ID: name,
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{Name: name},
			},
		})
	}
	return services, nil
}

func (cli *fakeClient) ServiceRemove(ctx context.Context, serviceID string) error {
	cli.removedIDs = append(cli.removedIDs, serviceID)
	return nil
}

func TestPruneServices(t *testing.T) {
	ctx := context.Background()
	namespace := convert.NewNamespace("foo")
	services := map[string]struct{}{
		"new":  {},
		"keep": {},
	}
	client := &fakeClient{serviceList: []string{"foo_keep", "foo_remove"}}
	dockerCli := test.NewFakeCli(client, &bytes.Buffer{})
	dockerCli.SetErr(&bytes.Buffer{})

	pruneServices(ctx, dockerCli, namespace, services)

	assert.DeepEqual(t, client.removedIDs, []string{"foo_remove"})
}
