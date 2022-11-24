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

	diffapi "github.com/containerd/containerd/api/services/diff/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/epoch"
	"github.com/containerd/containerd/protobuf"
	ptypes "github.com/containerd/containerd/protobuf/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DiffService handles the computation and application of diffs
type DiffService interface {
	diff.Comparer
	diff.Applier
}

// NewDiffServiceFromClient returns a new diff service which communicates
// over a GRPC connection.
func NewDiffServiceFromClient(client diffapi.DiffClient) DiffService {
	return &diffRemote{
		client: client,
	}
}

type diffRemote struct {
	client diffapi.DiffClient
}

func (r *diffRemote) Apply(ctx context.Context, desc ocispec.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (ocispec.Descriptor, error) {
	var config diff.ApplyConfig
	for _, opt := range opts {
		if err := opt(ctx, desc, &config); err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	payloads := make(map[string]*ptypes.Any)
	for k, v := range config.ProcessorPayloads {
		payloads[k] = protobuf.FromAny(v)
	}

	req := &diffapi.ApplyRequest{
		Diff:     fromDescriptor(desc),
		Mounts:   fromMounts(mounts),
		Payloads: payloads,
	}
	resp, err := r.client.Apply(ctx, req)
	if err != nil {
		return ocispec.Descriptor{}, errdefs.FromGRPC(err)
	}
	return toDescriptor(resp.Applied), nil
}

func (r *diffRemote) Compare(ctx context.Context, a, b []mount.Mount, opts ...diff.Opt) (ocispec.Descriptor, error) {
	var config diff.Config
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return ocispec.Descriptor{}, err
		}
	}
	if tm := epoch.FromContext(ctx); tm != nil && config.SourceDateEpoch == nil {
		config.SourceDateEpoch = tm
	}
	var sourceDateEpoch *timestamppb.Timestamp
	if config.SourceDateEpoch != nil {
		sourceDateEpoch = timestamppb.New(*config.SourceDateEpoch)
	}
	req := &diffapi.DiffRequest{
		Left:            fromMounts(a),
		Right:           fromMounts(b),
		MediaType:       config.MediaType,
		Ref:             config.Reference,
		Labels:          config.Labels,
		SourceDateEpoch: sourceDateEpoch,
	}
	resp, err := r.client.Diff(ctx, req)
	if err != nil {
		return ocispec.Descriptor{}, errdefs.FromGRPC(err)
	}
	return toDescriptor(resp.Diff), nil
}

func toDescriptor(d *types.Descriptor) ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType:   d.MediaType,
		Digest:      digest.Digest(d.Digest),
		Size:        d.Size,
		Annotations: d.Annotations,
	}
}

func fromDescriptor(d ocispec.Descriptor) *types.Descriptor {
	return &types.Descriptor{
		MediaType:   d.MediaType,
		Digest:      d.Digest.String(),
		Size:        d.Size,
		Annotations: d.Annotations,
	}
}

func fromMounts(mounts []mount.Mount) []*types.Mount {
	apiMounts := make([]*types.Mount, len(mounts))
	for i, m := range mounts {
		apiMounts[i] = &types.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Target:  m.Target,
			Options: m.Options,
		}
	}
	return apiMounts
}
