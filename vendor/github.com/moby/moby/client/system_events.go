package client

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/versions"
	"github.com/moby/moby/client/internal/timestamp"
)

// EventsListOptions holds parameters to filter events with.
type EventsListOptions struct {
	Since   string
	Until   string
	Filters filters.Args
}

// Events returns a stream of events in the daemon. It's up to the caller to close the stream
// by cancelling the context. Once the stream has been completely read an [io.EOF] error is
// sent over the error channel. If an error is sent, all processing is stopped. It's up
// to the caller to reopen the stream in the event of an error by reinvoking this method.
func (cli *Client) Events(ctx context.Context, options EventsListOptions) (<-chan events.Message, <-chan error) {
	messages := make(chan events.Message)
	errs := make(chan error, 1)

	started := make(chan struct{})
	go func() {
		defer close(errs)

		// Make sure we negotiated (if the client is configured to do so),
		// as code below contains API-version specific handling of options.
		//
		// Normally, version-negotiation (if enabled) would not happen until
		// the API request is made.
		if err := cli.checkVersion(ctx); err != nil {
			close(started)
			errs <- err
			return
		}
		query, err := buildEventsQueryParams(cli.version, options)
		if err != nil {
			close(started)
			errs <- err
			return
		}

		resp, err := cli.get(ctx, "/events", query, nil)
		if err != nil {
			close(started)
			errs <- err
			return
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)

		close(started)
		for {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			default:
				var event events.Message
				if err := decoder.Decode(&event); err != nil {
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

	return messages, errs
}

func buildEventsQueryParams(cliVersion string, options EventsListOptions) (url.Values, error) {
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

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return nil, err
		}
		if cliVersion != "" && versions.LessThan(cliVersion, "1.22") {
			legacyFormat, err := encodeLegacyFilters(filterJSON)
			if err != nil {
				return nil, err
			}
			filterJSON = legacyFormat
		}
		query.Set("filters", filterJSON)
	}

	return query, nil
}
