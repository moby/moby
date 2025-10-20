package client

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client/internal"
	"github.com/moby/moby/client/internal/timestamp"
)

// EventsListOptions holds parameters to filter events with.
type EventsListOptions struct {
	Since   string
	Until   string
	Filters Filters
}

// EventsResult holds the result of an Events query.
type EventsResult struct {
	Messages <-chan events.Message
	Err      <-chan error
}

// Events returns a stream of events in the daemon. It's up to the caller to close the stream
// by cancelling the context. Once the stream has been completely read an [io.EOF] error is
// sent over the error channel. If an error is sent, all processing is stopped. It's up
// to the caller to reopen the stream in the event of an error by reinvoking this method.
func (cli *Client) Events(ctx context.Context, options EventsListOptions) EventsResult {
	messages := make(chan events.Message)
	errs := make(chan error, 1)

	started := make(chan struct{})
	go func() {
		defer close(errs)

		query, err := buildEventsQueryParams(options)
		if err != nil {
			close(started)
			errs <- err
			return
		}

		headers := http.Header{}
		headers.Add("Accept", types.MediaTypeJSONSequence)
		headers.Add("Accept", types.MediaTypeNDJSON)
		resp, err := cli.get(ctx, "/events", query, headers)
		if err != nil {
			close(started)
			errs <- err
			return
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")
		decoder := internal.NewJSONStreamDecoder(resp.Body, contentType)

		close(started)
		for {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			default:
				var event events.Message
				if err := decoder(&event); err != nil {
					errs <- err
					return
				}

				select {
				case messages <- event:
				case <-ctx.Done():
					errs <- ctx.Err()
					return
				}
			}
		}
	}()
	<-started

	return EventsResult{
		Messages: messages,
		Err:      errs,
	}
}

func buildEventsQueryParams(options EventsListOptions) (url.Values, error) {
	query := url.Values{}
	ref := time.Now()

	if options.Since != "" {
		ts, err := timestamp.GetTimestamp(options.Since, ref)
		if err != nil {
			return nil, err
		}
		query.Set("since", ts)
	}

	if options.Until != "" {
		ts, err := timestamp.GetTimestamp(options.Until, ref)
		if err != nil {
			return nil, err
		}
		query.Set("until", ts)
	}

	options.Filters.updateURLValues(query)

	return query, nil
}
