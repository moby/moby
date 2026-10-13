package cluster

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
	executorpkg "github.com/moby/moby/v2/daemon/cluster/executor"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"github.com/opencontainers/go-digest"
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

type fakeRepository struct {
	distribution.Repository
	tags distribution.TagService
}

func (r fakeRepository) Tags(context.Context) distribution.TagService {
	return r.tags
}

type fakeTagService struct {
	distribution.TagService
	digest digest.Digest
	err    error
	calls  *int
}

func (s fakeTagService) Get(context.Context, string) (distribution.Descriptor, error) {
	*s.calls++
	return distribution.Descriptor{Digest: s.digest}, s.err
}

type fakeImageBackend struct {
	executorpkg.ImageBackend
	repos []distribution.Repository
}

func (b fakeImageBackend) GetRepositories(context.Context, reference.Named, *registry.AuthConfig) ([]distribution.Repository, error) {
	return b.repos, nil
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

// TestImageWithDigestString verifies that repository lookups stop at the first
// successful digest resolution and return an error if all lookups fail.
//
// Regression test for https://github.com/moby/moby/issues/53929
func TestImageWithDigestString(t *testing.T) {
	dgst := digest.FromString("manifest")
	lookupErr := errors.New("lookup failed")

	tests := []struct {
		doc      string
		errs     []error
		expCalls []int
		expErr   error
	}{
		{
			doc:      "mirror succeeds",
			errs:     []error{nil, lookupErr},
			expCalls: []int{1, 0},
		},
		{
			doc:      "upstream succeeds",
			errs:     []error{lookupErr, nil},
			expCalls: []int{1, 1},
		},
		{
			doc:      "both fail",
			errs:     []error{lookupErr, lookupErr},
			expCalls: []int{1, 1},
			expErr:   lookupErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			calls := make([]int, len(tc.errs))
			repos := make([]distribution.Repository, len(tc.errs))

			for i, err := range tc.errs {
				repos[i] = fakeRepository{
					tags: fakeTagService{
						digest: dgst,
						err:    err,
						calls:  &calls[i],
					},
				}
			}

			cluster := &Cluster{
				config: Config{ImageBackend: fakeImageBackend{repos: repos}},
			}
			got, err := cluster.imageWithDigestString(t.Context(), "busybox:latest", nil)

			if tc.expErr != nil {
				assert.ErrorIs(t, err, tc.expErr)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, got, "busybox:latest@"+dgst.String())
			}
			assert.DeepEqual(t, calls, tc.expCalls)
		})
	}
}
