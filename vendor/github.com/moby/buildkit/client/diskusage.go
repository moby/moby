package client

import (
	"context"
	"sort"
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/pkg/errors"
)

type UsageInfo struct {
	ID      string
	Mutable bool
	InUse   bool
	Size    int64

	CreatedAt   time.Time
	LastUsedAt  *time.Time
	UsageCount  int
	Parent      string
	Description string
}

func (c *Client) DiskUsage(ctx context.Context, opts ...DiskUsageOption) ([]*UsageInfo, error) {
	info := &DiskUsageInfo{}
	for _, o := range opts {
		o(info)
	}

	req := &controlapi.DiskUsageRequest{Filter: info.Filter}
	resp, err := c.controlClient().DiskUsage(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call diskusage")
	}

	var du []*UsageInfo

	for _, d := range resp.Record {
		du = append(du, &UsageInfo{
			ID:          d.ID,
			Mutable:     d.Mutable,
			InUse:       d.InUse,
			Size:        d.Size_,
			Parent:      d.Parent,
			CreatedAt:   d.CreatedAt,
			Description: d.Description,
			UsageCount:  int(d.UsageCount),
			LastUsedAt:  d.LastUsedAt,
		})
	}

	sort.Slice(du, func(i, j int) bool {
		if du[i].Size == du[j].Size {
			return du[i].ID > du[j].ID
		}
		return du[i].Size > du[j].Size
	})

	return du, nil
}

type DiskUsageOption func(*DiskUsageInfo)

type DiskUsageInfo struct {
	Filter string
}

func WithFilter(f string) DiskUsageOption {
	return func(di *DiskUsageInfo) {
		di.Filter = f
	}
}
