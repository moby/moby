package client

import (
	"context"
	"io"
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/pkg/errors"
)

func (c *Client) Prune(ctx context.Context, ch chan UsageInfo, opts ...PruneOption) error {
	info := &PruneInfo{}
	for _, o := range opts {
		o.SetPruneOption(info)
	}

	req := &controlapi.PruneRequest{
		Filter:       info.Filter,
		KeepDuration: int64(info.KeepDuration),
		KeepBytes:    int64(info.KeepBytes),
	}
	if info.All {
		req.All = true
	}
	cl, err := c.controlClient().Prune(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed to call prune")
	}

	for {
		d, err := cl.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if ch != nil {
			ch <- UsageInfo{
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
			}
		}
	}
}

type PruneOption interface {
	SetPruneOption(*PruneInfo)
}

type PruneInfo struct {
	Filter       []string      `json:"filter"`
	All          bool          `json:"all"`
	KeepDuration time.Duration `json:"keepDuration"`
	KeepBytes    int64         `json:"keepBytes"`
}

type pruneOptionFunc func(*PruneInfo)

func (f pruneOptionFunc) SetPruneOption(pi *PruneInfo) {
	f(pi)
}

var PruneAll = pruneOptionFunc(func(pi *PruneInfo) {
	pi.All = true
})

func WithKeepOpt(duration time.Duration, bytes int64) PruneOption {
	return pruneOptionFunc(func(pi *PruneInfo) {
		pi.KeepDuration = duration
		pi.KeepBytes = bytes
	})
}
