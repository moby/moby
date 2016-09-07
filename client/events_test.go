package client

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

func TestEventsErrorInOptions(t *testing.T) {
	errorCases := []struct {
		options       types.EventsOptions
		expectedError string
	}{
		{
			options: types.EventsOptions{
				Since: "2006-01-02TZ",
			},
			expectedError: `parsing time "2006-01-02TZ"`,
		},
		{
			options: types.EventsOptions{
				Until: "2006-01-02TZ",
			},
			expectedError: `parsing time "2006-01-02TZ"`,
		},
	}
	for _, e := range errorCases {
		client := &Client{
			transport: newMockClient(nil, errorMock(http.StatusInternalServerError, "Server error")),
		}
		_, err := client.Events(context.Background(), e.options)
		if err == nil || !strings.Contains(err.Error(), e.expectedError) {
			t.Fatalf("expected a error %q, got %v", e.expectedError, err)
		}
	}
}

func TestEventsErrorFromServer(t *testing.T) {
	client := &Client{
		transport: newMockClient(nil, errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.Events(context.Background(), types.EventsOptions{})
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}
}

func TestEvents(t *testing.T) {
	expectedURL := "/events"

	filters := filters.NewArgs()
	filters.Add("label", "label1")
	filters.Add("label", "label2")
	expectedFiltersJSON := `{"label":{"label1":true,"label2":true}}`

	eventsCases := []struct {
		options             types.EventsOptions
		expectedQueryParams map[string]string
	}{
		{
			options: types.EventsOptions{
				Since: "invalid but valid",
			},
			expectedQueryParams: map[string]string{
				"since": "invalid but valid",
			},
		},
		{
			options: types.EventsOptions{
				Until: "invalid but valid",
			},
			expectedQueryParams: map[string]string{
				"until": "invalid but valid",
			},
		},
		{
			options: types.EventsOptions{
				Filters: filters,
			},
			expectedQueryParams: map[string]string{
				"filters": expectedFiltersJSON,
			},
		},
	}

	for _, eventsCase := range eventsCases {
		client := &Client{
			transport: newMockClient(nil, func(req *http.Request) (*http.Response, error) {
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
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte("response"))),
				}, nil
			}),
		}
		body, err := client.Events(context.Background(), eventsCase.options)
		if err != nil {
			t.Fatal(err)
		}
		defer body.Close()
		content, err := ioutil.ReadAll(body)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "response" {
			t.Fatalf("expected response to contain 'response', got %s", string(content))
		}
	}
}
