/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package containerd

import (
	"context"
	"errors"
	"io"

	containersapi "github.com/containerd/containerd/api/services/containers/v1"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/protobuf"
	ptypes "github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/typeurl/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type remoteContainers struct {
	client containersapi.ContainersClient
}

var _ containers.Store = &remoteContainers{}

// NewRemoteContainerStore returns the container Store connected with the provided client
func NewRemoteContainerStore(client containersapi.ContainersClient) containers.Store {
	return &remoteContainers{
		client: client,
	}
}

func (r *remoteContainers) Get(ctx context.Context, id string) (containers.Container, error) {
	resp, err := r.client.Get(ctx, &containersapi.GetContainerRequest{
		ID: id,
	})
	if err != nil {
		return containers.Container{}, errdefs.FromGRPC(err)
	}

	return containerFromProto(resp.Container), nil
}

func (r *remoteContainers) List(ctx context.Context, filters ...string) ([]containers.Container, error) {
	containers, err := r.stream(ctx, filters...)
	if err != nil {
		if err == errStreamNotAvailable {
			return r.list(ctx, filters...)
		}
		return nil, err
	}
	return containers, nil
}

func (r *remoteContainers) list(ctx context.Context, filters ...string) ([]containers.Container, error) {
	resp, err := r.client.List(ctx, &containersapi.ListContainersRequest{
		Filters: filters,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return containersFromProto(resp.Containers), nil
}

var errStreamNotAvailable = errors.New("streaming api not available")

func (r *remoteContainers) stream(ctx context.Context, filters ...string) ([]containers.Container, error) {
	session, err := r.client.ListStream(ctx, &containersapi.ListContainersRequest{
		Filters: filters,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	var containers []containers.Container
	for {
		c, err := session.Recv()
		if err != nil {
			if err == io.EOF {
				return containers, nil
			}
			if s, ok := status.FromError(err); ok {
				if s.Code() == codes.Unimplemented {
					return nil, errStreamNotAvailable
				}
			}
			return nil, errdefs.FromGRPC(err)
		}
		select {
		case <-ctx.Done():
			return containers, ctx.Err()
		default:
			containers = append(containers, containerFromProto(c.Container))
		}
	}
}

func (r *remoteContainers) Create(ctx context.Context, container containers.Container) (containers.Container, error) {
	created, err := r.client.Create(ctx, &containersapi.CreateContainerRequest{
		Container: containerToProto(&container),
	})
	if err != nil {
		return containers.Container{}, errdefs.FromGRPC(err)
	}

	return containerFromProto(created.Container), nil

}

func (r *remoteContainers) Update(ctx context.Context, container containers.Container, fieldpaths ...string) (containers.Container, error) {
	var updateMask *ptypes.FieldMask
	if len(fieldpaths) > 0 {
		updateMask = &ptypes.FieldMask{
			Paths: fieldpaths,
		}
	}

	updated, err := r.client.Update(ctx, &containersapi.UpdateContainerRequest{
		Container:  containerToProto(&container),
		UpdateMask: updateMask,
	})
	if err != nil {
		return containers.Container{}, errdefs.FromGRPC(err)
	}

	return containerFromProto(updated.Container), nil

}

func (r *remoteContainers) Delete(ctx context.Context, id string) error {
	_, err := r.client.Delete(ctx, &containersapi.DeleteContainerRequest{
		ID: id,
	})

	return errdefs.FromGRPC(err)

}

func containerToProto(container *containers.Container) *containersapi.Container {
	extensions := make(map[string]*ptypes.Any)
	for k, v := range container.Extensions {
		extensions[k] = protobuf.FromAny(v)
	}
	return &containersapi.Container{
		ID:     container.ID,
		Labels: container.Labels,
		Image:  container.Image,
		Runtime: &containersapi.Container_Runtime{
			Name:    container.Runtime.Name,
			Options: protobuf.FromAny(container.Runtime.Options),
		},
		Spec:        protobuf.FromAny(container.Spec),
		Snapshotter: container.Snapshotter,
		SnapshotKey: container.SnapshotKey,
		Extensions:  extensions,
		Sandbox:     container.SandboxID,
	}
}

func containerFromProto(containerpb *containersapi.Container) containers.Container {
	var runtime containers.RuntimeInfo
	if containerpb.Runtime != nil {
		runtime = containers.RuntimeInfo{
			Name:    containerpb.Runtime.Name,
			Options: containerpb.Runtime.Options,
		}
	}
	extensions := make(map[string]typeurl.Any)
	for k, v := range containerpb.Extensions {
		v := v
		extensions[k] = v
	}
	return containers.Container{
		ID:          containerpb.ID,
		Labels:      containerpb.Labels,
		Image:       containerpb.Image,
		Runtime:     runtime,
		Spec:        containerpb.Spec,
		Snapshotter: containerpb.Snapshotter,
		SnapshotKey: containerpb.SnapshotKey,
		CreatedAt:   protobuf.FromTimestamp(containerpb.CreatedAt),
		UpdatedAt:   protobuf.FromTimestamp(containerpb.UpdatedAt),
		Extensions:  extensions,
		SandboxID:   containerpb.Sandbox,
	}
}

func containersFromProto(containerspb []*containersapi.Container) []containers.Container {
	var containers []containers.Container

	for _, container := range containerspb {
		container := container
		containers = append(containers, containerFromProto(container))
	}

	return containers
}
