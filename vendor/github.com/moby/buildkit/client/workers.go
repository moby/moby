package client

import (
	"context"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/solver/pb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type WorkerInfo struct {
	ID        string
	Labels    map[string]string
	Platforms []specs.Platform
}

func (c *Client) ListWorkers(ctx context.Context, opts ...ListWorkersOption) ([]*WorkerInfo, error) {
	info := &ListWorkersInfo{}
	for _, o := range opts {
		o(info)
	}

	req := &controlapi.ListWorkersRequest{Filter: info.Filter}
	resp, err := c.controlClient().ListWorkers(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workers")
	}

	var wi []*WorkerInfo

	for _, w := range resp.Record {
		wi = append(wi, &WorkerInfo{
			ID:        w.ID,
			Labels:    w.Labels,
			Platforms: pb.ToSpecPlatforms(w.Platforms),
		})
	}

	return wi, nil
}

type ListWorkersOption func(*ListWorkersInfo)

type ListWorkersInfo struct {
	Filter []string
}

func WithWorkerFilter(f []string) ListWorkersOption {
	return func(wi *ListWorkersInfo) {
		wi.Filter = f
	}
}
