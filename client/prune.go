package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"

	"github.com/moby/moby/api/types/jsonstream"
)

// PruneProgress describes an object successfully removed by a prune request.
// ID has the same representation as the corresponding entry in the prune report.
// Action is "deleted" or "untagged" (for image references).
type PruneProgress struct {
	ID     string
	Action string
}

func (cli *Client) prune(ctx context.Context, path string, query url.Values, onProgress func(PruneProgress) error, report any) error {
	if onProgress != nil {
		if err := cli.requiresVersion(ctx, "1.56", "prune progress"); err != nil {
			return err
		}
		query.Set("stream", "true")
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
	}
	resp, err := cli.post(ctx, path, query, nil, nil)
	if onProgress == nil {
		defer ensureReaderClosed(resp)
	} else if resp != nil && resp.Body != nil {
		// Do not drain a live stream: the daemon may be waiting for cancellation.
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}
	if onProgress == nil {
		return json.NewDecoder(resp.Body).Decode(report)
	}
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return fmt.Errorf("invalid prune response content type: %w", err)
	}
	if contentType == "application/json" {
		// Earlier daemon builds may support this API version but ignore stream.
		// Decode their report without retrying the destructive request.
		return json.NewDecoder(resp.Body).Decode(report)
	}
	if contentType != "application/jsonl" {
		return errors.New("daemon did not return a prune progress stream")
	}
	return decodePruneStream(resp.Body, onProgress, report)
}

func decodePruneStream(reader io.Reader, onProgress func(PruneProgress) error, report any) error {
	dec := json.NewDecoder(reader)
	for {
		var message jsonstream.Message
		if err := dec.Decode(&message); err != nil {
			if errors.Is(err, io.EOF) {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		if message.Error != nil {
			return fmt.Errorf("prune failed: %s", message.Error.Message)
		}
		if message.Aux != nil {
			return json.Unmarshal(*message.Aux, report)
		}
		if message.ID != "" {
			if err := onProgress(PruneProgress{ID: message.ID, Action: message.Status}); err != nil {
				return err
			}
		}
	}
}
