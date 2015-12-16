package lib

import (
	"io"
	"net/url"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	timetypes "github.com/docker/docker/api/types/time"
)

// Events returns a stream of events in the daemon in a ReadCloser.
// It's up to the caller to close the stream.
func (cli *Client) Events(options types.EventsOptions) (io.ReadCloser, error) {
	query := url.Values{}
	ref := time.Now()

	if options.Since != "" {
		ts, err := timetypes.GetTimestamp(options.Since, ref)
		if err != nil {
			return nil, err
		}
		query.Set("since", ts)
	}
	if options.Until != "" {
		ts, err := timetypes.GetTimestamp(options.Until, ref)
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

	serverResponse, err := cli.get("/events", query, nil)
	if err != nil {
		return nil, err
	}
	return serverResponse.body, nil
}
