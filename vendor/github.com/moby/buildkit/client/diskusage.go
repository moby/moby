package client

import (
	"cmp"
	"context"
	"slices"
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

	req := &controlapi.DiskUsageRequest{Filter: info.Filter, AgeLimit: int64(info.AgeLimit)}
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
			Size:        d.Size,
			Parents:     d.Parents,
			CreatedAt:   d.CreatedAt.AsTime(),
			Description: d.Description,
			UsageCount:  int(d.UsageCount),
			LastUsedAt: func() *time.Time {
				if d.LastUsedAt != nil {
					ts := d.LastUsedAt.AsTime()
					return &ts
				}
				return nil
			}(),
			RecordType: UsageRecordType(d.RecordType),
			Shared:     d.Shared,
		})
	}

	slices.SortFunc(du, func(a, b *UsageInfo) int {
		return cmp.Or(cmp.Compare(a.Size, b.Size), cmp.Compare(a.ID, b.ID))
	})
	return du, nil
}

type DiskUsageOption interface {
	SetDiskUsageOption(*DiskUsageInfo)
}

type DiskUsageInfo struct {
	Filter   []string
	AgeLimit time.Duration
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

type diskUsageOptionFunc func(*DiskUsageInfo)

func (f diskUsageOptionFunc) SetDiskUsageOption(info *DiskUsageInfo) {
	f(info)
}

func WithAgeLimit(age time.Duration) DiskUsageOption {
	return diskUsageOptionFunc(func(info *DiskUsageInfo) {
		info.AgeLimit = age
	})
}
