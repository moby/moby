package client

import (
	"context"
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	apitypes "github.com/moby/buildkit/api/types"
	"github.com/moby/buildkit/solver/pb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// WorkerInfo contains information about a worker
type WorkerInfo struct {
	ID        string
	Labels    map[string]string
	Platforms []ocispecs.Platform
	GCPolicy  []PruneInfo
}

// ListWorkers lists all active workers
func (c *Client) ListWorkers(ctx context.Context, opts ...ListWorkersOption) ([]*WorkerInfo, error) {
	info := &ListWorkersInfo{}
	for _, o := range opts {
		o.SetListWorkersOption(info)
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
			GCPolicy:  fromAPIGCPolicy(w.GCPolicy),
		})
	}

	return wi, nil
}

// ListWorkersOption is an option for a worker list query
type ListWorkersOption interface {
	SetListWorkersOption(*ListWorkersInfo)
}

// ListWorkersInfo is a payload for worker list query
type ListWorkersInfo struct {
	Filter []string
}

func fromAPIGCPolicy(in []*apitypes.GCPolicy) []PruneInfo {
	out := make([]PruneInfo, 0, len(in))
	for _, p := range in {
		out = append(out, PruneInfo{
			All:          p.All,
			Filter:       p.Filters,
			KeepDuration: time.Duration(p.KeepDuration),
			KeepBytes:    p.KeepBytes,
		})
	}
	return out
}
