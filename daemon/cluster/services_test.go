package cluster

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types/swarm"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"google.golang.org/grpc"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type fakeServiceCreator struct {
	swarmapi.ControlClient
	name string
}

func (f *fakeServiceCreator) CreateService(_ context.Context, req *swarmapi.CreateServiceRequest, _ ...grpc.CallOption) (*swarmapi.CreateServiceResponse, error) {
	f.name = req.Spec.Annotations.Name
	return &swarmapi.CreateServiceResponse{Service: &swarmapi.Service{ID: "serviceid"}}, nil
}

func TestCreateServiceName(t *testing.T) {
	for _, name := range []string{"", "chosen_name"} {
		t.Run(name, func(t *testing.T) {
			client := new(fakeServiceCreator)
			cluster := &Cluster{nr: &nodeRunner{nodeState: nodeState{controlClient: client}}}
			resp, err := cluster.CreateService(swarm.ServiceSpec{
				Annotations: swarm.Annotations{Name: name},
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: &swarm.ContainerSpec{Image: "image"},
				},
			}, "", false)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(resp.ID, "serviceid"))
			if name == "" {
				assert.Check(t, client.name != "")
			} else {
				assert.Check(t, is.Equal(client.name, name))
			}
		})
	}
}
