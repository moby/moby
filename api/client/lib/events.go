package lib

import (
	"io"
	"net/url"
	"time"

	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/timeutils"
)

// EventsOptions hold parameters to filter events with.
type EventsOptions struct {
	Since   string
	Until   string
	Filters filters.Args
}

// Events returns a stream of events in the daemon in a ReadCloser.
// It's up to the caller to close the stream.
func (cli *Client) Events(options EventsOptions) (io.ReadCloser, error) {
	var query url.Values
	ref := time.Now()

	if options.Since != "" {
		ts, err := timeutils.GetTimestamp(options.Since, ref)
		if err != nil {
			return nil, err
		}
		query.Set("since", ts)
	}
	if options.Until != "" {
		ts, err := timeutils.GetTimestamp(options.Until, ref)
		if err != nil {
			return nil, err
		}
		query.Set("until", ts)
	}
	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToParam(options.Filters)
		if err != nil {
			return nil, err
		}
		query.Set("filters", filterJSON)
	}

	serverResponse, err := cli.GET("/events", query, nil)
	if err != nil {
		return nil, err
	}
	return serverResponse.body, nil
}
