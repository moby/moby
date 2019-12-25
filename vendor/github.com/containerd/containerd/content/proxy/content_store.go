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

package proxy

import (
	"context"
	"io"

	contentapi "github.com/containerd/containerd/api/services/content/v1"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	protobuftypes "github.com/gogo/protobuf/types"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type proxyContentStore struct {
	client contentapi.ContentClient
}

// NewContentStore returns a new content store which communicates over a GRPC
// connection using the containerd content GRPC API.
func NewContentStore(client contentapi.ContentClient) content.Store {
	return &proxyContentStore{
		client: client,
	}
}

func (pcs *proxyContentStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	resp, err := pcs.client.Info(ctx, &contentapi.InfoRequest{
		Digest: dgst,
	})
	if err != nil {
		return content.Info{}, errdefs.FromGRPC(err)
	}

	return infoFromGRPC(resp.Info), nil
}

func (pcs *proxyContentStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	session, err := pcs.client.List(ctx, &contentapi.ListContentRequest{
		Filters: filters,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}

	for {
		msg, err := session.Recv()
		if err != nil {
			if err != io.EOF {
				return errdefs.FromGRPC(err)
			}

			break
		}

		for _, info := range msg.Info {
			if err := fn(infoFromGRPC(info)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (pcs *proxyContentStore) Delete(ctx context.Context, dgst digest.Digest) error {
	if _, err := pcs.client.Delete(ctx, &contentapi.DeleteContentRequest{
		Digest: dgst,
	}); err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

// ReaderAt ignores MediaType.
func (pcs *proxyContentStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	i, err := pcs.Info(ctx, desc.Digest)
	if err != nil {
		return nil, err
	}

	return &remoteReaderAt{
		ctx:    ctx,
		digest: desc.Digest,
		size:   i.Size,
		client: pcs.client,
	}, nil
}

func (pcs *proxyContentStore) Status(ctx context.Context, ref string) (content.Status, error) {
	resp, err := pcs.client.Status(ctx, &contentapi.StatusRequest{
		Ref: ref,
	})
	if err != nil {
		return content.Status{}, errdefs.FromGRPC(err)
	}

	status := resp.Status
	return content.Status{
		Ref:       status.Ref,
		StartedAt: status.StartedAt,
		UpdatedAt: status.UpdatedAt,
		Offset:    status.Offset,
		Total:     status.Total,
		Expected:  status.Expected,
	}, nil
}

func (pcs *proxyContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	resp, err := pcs.client.Update(ctx, &contentapi.UpdateRequest{
		Info: infoToGRPC(info),
		UpdateMask: &protobuftypes.FieldMask{
			Paths: fieldpaths,
		},
	})
	if err != nil {
		return content.Info{}, errdefs.FromGRPC(err)
	}
	return infoFromGRPC(resp.Info), nil
}

func (pcs *proxyContentStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	resp, err := pcs.client.ListStatuses(ctx, &contentapi.ListStatusesRequest{
		Filters: filters,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	var statuses []content.Status
	for _, status := range resp.Statuses {
		statuses = append(statuses, content.Status{
			Ref:       status.Ref,
			StartedAt: status.StartedAt,
			UpdatedAt: status.UpdatedAt,
			Offset:    status.Offset,
			Total:     status.Total,
			Expected:  status.Expected,
		})
	}

	return statuses, nil
}

// Writer ignores MediaType.
func (pcs *proxyContentStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	var wOpts content.WriterOpts
	for _, opt := range opts {
		if err := opt(&wOpts); err != nil {
			return nil, err
		}
	}
	wrclient, offset, err := pcs.negotiate(ctx, wOpts.Ref, wOpts.Desc.Size, wOpts.Desc.Digest)
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	return &remoteWriter{
		ref:    wOpts.Ref,
		client: wrclient,
		offset: offset,
	}, nil
}

// Abort implements asynchronous abort. It starts a new write session on the ref l
func (pcs *proxyContentStore) Abort(ctx context.Context, ref string) error {
	if _, err := pcs.client.Abort(ctx, &contentapi.AbortRequest{
		Ref: ref,
	}); err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (pcs *proxyContentStore) negotiate(ctx context.Context, ref string, size int64, expected digest.Digest) (contentapi.Content_WriteClient, int64, error) {
	wrclient, err := pcs.client.Write(ctx)
	if err != nil {
		return nil, 0, err
	}

	if err := wrclient.Send(&contentapi.WriteContentRequest{
		Action:   contentapi.WriteActionStat,
		Ref:      ref,
		Total:    size,
		Expected: expected,
	}); err != nil {
		return nil, 0, err
	}

	resp, err := wrclient.Recv()
	if err != nil {
		return nil, 0, err
	}

	return wrclient, resp.Offset, nil
}

func infoToGRPC(info content.Info) contentapi.Info {
	return contentapi.Info{
		Digest:    info.Digest,
		Size_:     info.Size,
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
		Labels:    info.Labels,
	}
}

func infoFromGRPC(info contentapi.Info) content.Info {
	return content.Info{
		Digest:    info.Digest,
		Size:      info.Size_,
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
		Labels:    info.Labels,
	}
}
