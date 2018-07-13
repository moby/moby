package client

import (
	"context"
	"io"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/pkg/errors"
)

func (c *Client) Prune(ctx context.Context, ch chan UsageInfo, opts ...PruneOption) error {
	info := &PruneInfo{}
	for _, o := range opts {
		o(info)
	}

	req := &controlapi.PruneRequest{}
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
				Parent:      d.Parent,
				CreatedAt:   d.CreatedAt,
				Description: d.Description,
				UsageCount:  int(d.UsageCount),
				LastUsedAt:  d.LastUsedAt,
			}
		}
	}
}

type PruneOption func(*PruneInfo)

type PruneInfo struct {
}
