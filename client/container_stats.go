package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client/containerstats"
	"github.com/moby/moby/client/internal/opts"
	internalshared "github.com/moby/moby/client/internal/shared"
)

// ContainerStats returns near realtime stats for a given container.
// It's up to the caller to close the [io.ReadCloser] returned.
func (cli *Client) ContainerStats(ctx context.Context, containerID string, options ...containerstats.Option) (containerstats.Output, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return nil, err
	}

	opts := &opts.ContainerStatsOptions{
		OutputStream: nil,
		OneShot:      nil,
	}
	for _, option := range options {
		err := option.ApplyContainerStatsOption(ctx, opts)
		if err != nil {
			return nil, err
		}
	}
	if opts.OneShot == nil && opts.OutputStream == nil {
		return nil, errors.New("must specify either oneshot or output stream to capture output")
	}

	query := url.Values{}
	query.Set("stream", "0")
	if opts.OutputStream != nil {
		query.Set("stream", "1")
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/stats", query, nil)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = resp.Body.Close()

		// TODO(vvoland): Should we close the output stream or leave that up to the caller?
		// It's convenient to close the output stream here so we don't force caller to do it,
		// but it's a bit inconsistent since the channel was created by the caller.
		if opts.OutputStream != nil {
			close(opts.OutputStream)
		}
	}()
	dec := json.NewDecoder(resp.Body)
	for dec.More() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var v containertypes.StatsResponse
		err = dec.Decode(&v)
		if err != nil {
			if err == io.EOF {
				break
			}

			err = fmt.Errorf("failed to decode stats response: %w", err)
			if opts.OutputStream == nil {
				return nil, err
			}
			opts.OutputStream <- internalshared.StreamItem{Error: err}
			continue
		}
		if opts.OneShot != nil {
			*opts.OneShot = v
			break
		}
		if opts.OutputStream != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			opts.OutputStream <- internalshared.StreamItem{Stats: &v}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
	}

	return struct{}{}, nil
}
