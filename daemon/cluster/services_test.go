package cluster

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types/swarm"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type fakeServiceCreator struct {
	swarmapi.ControlClient

	conflicts int
	err       error
	names     []string
}

func (f *fakeServiceCreator) CreateService(_ context.Context, req *swarmapi.CreateServiceRequest, _ ...grpc.CallOption) (*swarmapi.CreateServiceResponse, error) {
	name := req.Spec.Annotations.Name
	f.names = append(f.names, name)
	if len(f.names) <= f.conflicts {
		if f.err != nil {
			return nil, f.err
		}
		return nil, status.Errorf(codes.AlreadyExists, "name conflicts with an existing object: service %s already exists", name)
	}
	return &swarmapi.CreateServiceResponse{Service: &swarmapi.Service{ID: "serviceid"}}, nil
}

func TestCreateServiceNameConflict(t *testing.T) {
	tests := []struct {
		doc       string
		name      string
		conflicts int
		err       error
		expCalls  int
		expErr    codes.Code
	}{
		{
			doc:       "generated name is redrawn on conflict",
			conflicts: 1,
			expCalls:  2,
		},
		{
			doc:       "gives up after six draws",
			conflicts: 100,
			expCalls:  6,
			expErr:    codes.AlreadyExists,
		},
		{
			doc:       "caller-chosen name is not redrawn",
			name:      "bold_lichterman",
			conflicts: 1,
			expCalls:  1,
			expErr:    codes.AlreadyExists,
		},
		{
			doc:       "other errors are not retried",
			conflicts: 1,
			err:       status.Error(codes.InvalidArgument, "invalid spec"),
			expCalls:  1,
			expErr:    codes.InvalidArgument,
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client := &fakeServiceCreator{conflicts: tc.conflicts, err: tc.err}
			cluster := &Cluster{nr: &nodeRunner{nodeState: nodeState{controlClient: client}}}
			resp, err := cluster.CreateService(swarm.ServiceSpec{
				Annotations: swarm.Annotations{Name: tc.name},
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: &swarm.ContainerSpec{Image: "image"},
				},
			}, "", false)

			assert.Check(t, is.Len(client.names, tc.expCalls))
			assert.Check(t, is.Equal(status.Code(err), tc.expErr))
			if tc.expErr == codes.OK {
				assert.Check(t, is.Equal(resp.ID, "serviceid"))
			}
			if len(client.names) > 1 {
				assert.Check(t, client.names[1] != client.names[0])
			}
		})
	}
}
