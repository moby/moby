package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/streams"
)

// StreamCreate creates a stream in the daemon host.
// A stream can be used for bi-directional communication with the daemon.
func (c *Client) StreamCreate(ctx context.Context, id string, spec streams.Spec) (streams.Stream, error) {
	var s streams.Stream
	q := url.Values{}
	q.Set("id", id)
	resp, err := c.post(ctx, "/streams/create", q, spec, nil)
	if err != nil {
		return s, err
	}
	defer ensureReaderClosed(resp)

	if err != nil {
		return s, err
	}
	err = json.NewDecoder(resp.body).Decode(&s)
	return s, err
}

// StreamInspect returns information about a stream.
func (c *Client) StreamInspect(ctx context.Context, id string) (streams.Stream, error) {
	var s streams.Stream
	resp, err := c.get(ctx, "/streams/"+id, nil, nil)
	if err != nil {
		return s, err
	}
	defer ensureReaderClosed(resp)

	if err != nil {
		return s, err
	}
	err = json.NewDecoder(resp.body).Decode(&s)
	return s, err
}

// StreamDelete deletes a stream.
func (c *Client) StreamDelete(ctx context.Context, id string) error {
	resp, err := c.delete(ctx, "/streams/"+id, nil, nil)
	if err != nil {
		return err
	}
	defer ensureReaderClosed(resp)

	return nil
}
