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
	"fmt"
	"io"

	contentapi "github.com/containerd/containerd/api/services/content/v1"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/ttrpc"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/pkg/protobuf"
	protobuftypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
)

type proxyContentStore struct {
	// client is the rpc content client
	// NOTE: ttrpc is used because it is the smaller interface shared with grpc
	client contentapi.TTRPCContentClient
}

// NewContentStore returns a new content store which communicates over a GRPC
// connection using the containerd content GRPC API.
func NewContentStore(client any) content.Store {
	switch c := client.(type) {
	case contentapi.ContentClient:
		return &proxyContentStore{
			client: convertClient{c},
		}
	case grpc.ClientConnInterface:
		return &proxyContentStore{
			client: convertClient{contentapi.NewContentClient(c)},
		}
	case contentapi.TTRPCContentClient:
		return &proxyContentStore{
			client: c,
		}
	case *ttrpc.Client:
		return &proxyContentStore{
			client: contentapi.NewTTRPCContentClient(c),
		}
	default:
		panic(fmt.Errorf("unsupported content client %T: %w", client, errdefs.ErrNotImplemented))
	}
}

func (pcs *proxyContentStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	resp, err := pcs.client.Info(ctx, &contentapi.InfoRequest{
		Digest: dgst.String(),
	})
	if err != nil {
		return content.Info{}, errgrpc.ToNative(err)
	}

	return infoFromGRPC(resp.Info), nil
}

func (pcs *proxyContentStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	session, err := pcs.client.List(ctx, &contentapi.ListContentRequest{
		Filters: filters,
	})
	if err != nil {
		return errgrpc.ToNative(err)
	}

	for {
		msg, err := session.Recv()
		if err != nil {
			if err != io.EOF {
				return errgrpc.ToNative(err)
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
		Digest: dgst.String(),
	}); err != nil {
		return errgrpc.ToNative(err)
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
		return content.Status{}, errgrpc.ToNative(err)
	}

	status := resp.Status
	return content.Status{
		Ref:       status.Ref,
		StartedAt: protobuf.FromTimestamp(status.StartedAt),
		UpdatedAt: protobuf.FromTimestamp(status.UpdatedAt),
		Offset:    status.Offset,
		Total:     status.Total,
		Expected:  digest.Digest(status.Expected),
	}, nil
}

func (pcs *proxyContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	resp, err := pcs.client.Update(ctx, &contentapi.UpdateRequest{
		Info: infoToGRPC(&info),
		UpdateMask: &protobuftypes.FieldMask{
			Paths: fieldpaths,
		},
	})
	if err != nil {
		return content.Info{}, errgrpc.ToNative(err)
	}
	return infoFromGRPC(resp.Info), nil
}

func (pcs *proxyContentStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	resp, err := pcs.client.ListStatuses(ctx, &contentapi.ListStatusesRequest{
		Filters: filters,
	})
	if err != nil {
		return nil, errgrpc.ToNative(err)
	}

	var statuses []content.Status
	for _, status := range resp.Statuses {
		statuses = append(statuses, content.Status{
			Ref:       status.Ref,
			StartedAt: protobuf.FromTimestamp(status.StartedAt),
			UpdatedAt: protobuf.FromTimestamp(status.UpdatedAt),
			Offset:    status.Offset,
			Total:     status.Total,
			Expected:  digest.Digest(status.Expected),
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
		return nil, errgrpc.ToNative(err)
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
		return errgrpc.ToNative(err)
	}

	return nil
}

func (pcs *proxyContentStore) negotiate(ctx context.Context, ref string, size int64, expected digest.Digest) (contentapi.TTRPCContent_WriteClient, int64, error) {
	wrclient, err := pcs.client.Write(ctx)
	if err != nil {
		return nil, 0, err
	}

	if err := wrclient.Send(&contentapi.WriteContentRequest{
		Action:   contentapi.WriteAction_STAT,
		Ref:      ref,
		Total:    size,
		Expected: expected.String(),
	}); err != nil {
		return nil, 0, err
	}

	resp, err := wrclient.Recv()
	if err != nil {
		return nil, 0, err
	}

	return wrclient, resp.Offset, nil
}

type convertClient struct {
	contentapi.ContentClient
}

func (c convertClient) Info(ctx context.Context, req *contentapi.InfoRequest) (*contentapi.InfoResponse, error) {
	return c.ContentClient.Info(ctx, req)
}

func (c convertClient) Update(ctx context.Context, req *contentapi.UpdateRequest) (*contentapi.UpdateResponse, error) {
	return c.ContentClient.Update(ctx, req)
}

type convertListClient struct {
	contentapi.Content_ListClient
}

func (c convertClient) List(ctx context.Context, req *contentapi.ListContentRequest) (contentapi.TTRPCContent_ListClient, error) {
	lc, err := c.ContentClient.List(ctx, req)
	if lc == nil {
		return nil, err
	}
	return convertListClient{lc}, err
}

func (c convertClient) Delete(ctx context.Context, req *contentapi.DeleteContentRequest) (*emptypb.Empty, error) {
	return c.ContentClient.Delete(ctx, req)
}

type convertReadClient struct {
	contentapi.Content_ReadClient
}

func (c convertClient) Read(ctx context.Context, req *contentapi.ReadContentRequest) (contentapi.TTRPCContent_ReadClient, error) {
	rc, err := c.ContentClient.Read(ctx, req)
	if rc == nil {
		return nil, err
	}
	return convertReadClient{rc}, err
}

func (c convertClient) Status(ctx context.Context, req *contentapi.StatusRequest) (*contentapi.StatusResponse, error) {
	return c.ContentClient.Status(ctx, req)
}

func (c convertClient) ListStatuses(ctx context.Context, req *contentapi.ListStatusesRequest) (*contentapi.ListStatusesResponse, error) {
	return c.ContentClient.ListStatuses(ctx, req)
}

type convertWriteClient struct {
	contentapi.Content_WriteClient
}

func (c convertClient) Write(ctx context.Context) (contentapi.TTRPCContent_WriteClient, error) {
	wc, err := c.ContentClient.Write(ctx)
	if wc == nil {
		return nil, err
	}
	return convertWriteClient{wc}, err
}

func (c convertClient) Abort(ctx context.Context, req *contentapi.AbortRequest) (*emptypb.Empty, error) {
	return c.ContentClient.Abort(ctx, req)
}

func infoToGRPC(info *content.Info) *contentapi.Info {
	return &contentapi.Info{
		Digest:    info.Digest.String(),
		Size:      info.Size,
		CreatedAt: protobuf.ToTimestamp(info.CreatedAt),
		UpdatedAt: protobuf.ToTimestamp(info.UpdatedAt),
		Labels:    info.Labels,
	}
}

func infoFromGRPC(info *contentapi.Info) content.Info {
	return content.Info{
		Digest:    digest.Digest(info.Digest),
		Size:      info.Size,
		CreatedAt: protobuf.FromTimestamp(info.CreatedAt),
		UpdatedAt: protobuf.FromTimestamp(info.UpdatedAt),
		Labels:    info.Labels,
	}
}
