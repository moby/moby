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
	Parents     []string
	Description string
	RecordType  UsageRecordType
	Shared      bool
}

func (c *Client) DiskUsage(ctx context.Context, opts ...DiskUsageOption) ([]*UsageInfo, error) {
	info := &DiskUsageInfo{}
	for _, o := range opts {
		o.SetDiskUsageOption(info)
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
			Parents:     d.Parents,
			CreatedAt:   d.CreatedAt,
			Description: d.Description,
			UsageCount:  int(d.UsageCount),
			LastUsedAt:  d.LastUsedAt,
			RecordType:  UsageRecordType(d.RecordType),
			Shared:      d.Shared,
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

type DiskUsageOption interface {
	SetDiskUsageOption(*DiskUsageInfo)
}

type DiskUsageInfo struct {
	Filter []string
}

type UsageRecordType string

const (
	UsageRecordTypeInternal    UsageRecordType = "internal"
	UsageRecordTypeFrontend    UsageRecordType = "frontend"
	UsageRecordTypeLocalSource UsageRecordType = "source.local"
	UsageRecordTypeGitCheckout UsageRecordType = "source.git.checkout"
	UsageRecordTypeCacheMount  UsageRecordType = "exec.cachemount"
	UsageRecordTypeRegular     UsageRecordType = "regular"
)
