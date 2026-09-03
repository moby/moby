package cluster

import (
	"context"
	"fmt"
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
		expNames  []string
		expErr    codes.Code
	}{
		{
			doc:       "generated name is redrawn on conflict",
			conflicts: 1,
			expNames:  []string{"generated-0", "generated-1"},
		},
		{
			doc:       "gives up after six draws",
			conflicts: 100,
			expNames:  []string{"generated-0", "generated-1", "generated-2", "generated-3", "generated-4", "generated-5"},
			expErr:    codes.AlreadyExists,
		},
		{
			doc:       "caller-chosen name is not redrawn",
			name:      "bold_lichterman",
			conflicts: 1,
			expNames:  []string{"bold_lichterman"},
			expErr:    codes.AlreadyExists,
		},
		{
			doc:       "other errors are not retried",
			conflicts: 1,
			err:       status.Error(codes.InvalidArgument, "invalid spec"),
			expNames:  []string{"generated-0"},
			expErr:    codes.InvalidArgument,
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client := &fakeServiceCreator{conflicts: tc.conflicts, err: tc.err}
			var generatedImages []string
			cluster := &Cluster{
				config: Config{GenerateServiceName: func(_ context.Context, retry int, image string) (string, error) {
					generatedImages = append(generatedImages, image)
					return fmt.Sprintf("generated-%d", retry), nil
				}},
				nr: &nodeRunner{nodeState: nodeState{controlClient: client}},
			}
			resp, err := cluster.CreateService(swarm.ServiceSpec{
				Annotations: swarm.Annotations{Name: tc.name},
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: &swarm.ContainerSpec{Image: "image"},
				},
			}, "", false)

			assert.Check(t, is.DeepEqual(client.names, tc.expNames))
			if tc.name == "" {
				assert.Check(t, is.Len(generatedImages, len(tc.expNames)))
				for _, image := range generatedImages {
					assert.Check(t, is.Equal(image, "image"))
				}
			} else {
				assert.Check(t, is.Len(generatedImages, 0))
			}
			assert.Check(t, is.Equal(status.Code(err), tc.expErr))
			if tc.expErr == codes.OK {
				assert.Check(t, is.Equal(resp.ID, "serviceid"))
			}
		})
	}
}
