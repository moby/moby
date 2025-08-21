package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/filters"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestEventsErrorInOptions(t *testing.T) {
	errorCases := []struct {
		options       EventsListOptions
		expectedError string
	}{
		{
			options: EventsListOptions{
				Since: "2006-01-02TZ",
			},
			expectedError: `parsing time "2006-01-02TZ"`,
		},
		{
			options: EventsListOptions{
				Until: "2006-01-02TZ",
			},
			expectedError: `parsing time "2006-01-02TZ"`,
		},
	}
	for _, tc := range errorCases {
		client := &Client{
			client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
		}
		_, errs := client.Events(context.Background(), tc.options)
		err := <-errs
		assert.Check(t, is.ErrorContains(err, tc.expectedError))
	}
}

func TestEventsErrorFromServer(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, errs := client.Events(context.Background(), EventsListOptions{})
	err := <-errs
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestEvents(t *testing.T) {
	const expectedURL = "/events"

	fltrs := filters.NewArgs(filters.Arg("type", string(events.ContainerEventType)))
	expectedFiltersJSON := fmt.Sprintf(`{"type":{%q:true}}`, events.ContainerEventType)

	eventsCases := []struct {
		options             EventsListOptions
		events              []events.Message
		expectedEvents      map[string]bool
		expectedQueryParams map[string]string
	}{
		{
			options: EventsListOptions{
				Filters: fltrs,
			},
			expectedQueryParams: map[string]string{
				"filters": expectedFiltersJSON,
			},
			events:         []events.Message{},
			expectedEvents: make(map[string]bool),
		},
		{
			options: EventsListOptions{
				Filters: fltrs,
			},
			expectedQueryParams: map[string]string{
				"filters": expectedFiltersJSON,
			},
			events: []events.Message{
				{
					Type:   events.BuilderEventType,
					Actor:  events.Actor{ID: "1"},
					Action: events.ActionCreate,
				},
				{
					Type:   events.BuilderEventType,
					Actor:  events.Actor{ID: "1"},
					Action: events.ActionDie,
				},
				{
					Type:   events.BuilderEventType,
					Actor:  events.Actor{ID: "1"},
					Action: events.ActionCreate,
				},
			},
			expectedEvents: map[string]bool{
				"1": true,
				"2": true,
				"3": true,
			},
		},
	}

	for _, eventsCase := range eventsCases {
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				query := req.URL.Query()

				for key, expected := range eventsCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}

				buffer := new(bytes.Buffer)

				for _, e := range eventsCase.events {
					b, _ := json.Marshal(e)
					buffer.Write(b)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(buffer),
				}, nil
			}),
		}

		messages, errs := client.Events(context.Background(), eventsCase.options)

	loop:
		for {
			select {
			case err := <-errs:
				if err != nil && !errors.Is(err, io.EOF) {
					t.Fatal(err)
				}

				break loop
			case e := <-messages:
				_, ok := eventsCase.expectedEvents[e.Actor.ID]
				assert.Check(t, ok, "event received not expected with action %s & id %s", e.Action, e.Actor.ID)
			}
		}
	}
}
