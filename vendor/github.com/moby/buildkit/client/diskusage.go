package client

import (
	"context"
	"sort"
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/pkg/errors"
)

type UsageInfo struct {
	ID      string `json:"id"`
	Mutable bool   `json:"mutable"`
	InUse   bool   `json:"inUse"`
	Size    int64  `json:"size"`

	CreatedAt   time.Time       `json:"createdAt"`
	LastUsedAt  *time.Time      `json:"lastUsedAt"`
	UsageCount  int             `json:"usageCount"`
	Parents     []string        `json:"parents"`
	Description string          `json:"description"`
	RecordType  UsageRecordType `json:"recordType"`
	Shared      bool            `json:"shared"`
}

func (c *Client) DiskUsage(ctx context.Context, opts ...DiskUsageOption) ([]*UsageInfo, error) {
	info := &DiskUsageInfo{}
	for _, o := range opts {
		o.SetDiskUsageOption(info)
	}

	req := &controlapi.DiskUsageRequest{Filter: info.Filter}
	resp, err := c.ControlClient().DiskUsage(ctx, req)
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
