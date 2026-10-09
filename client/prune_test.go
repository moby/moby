package client

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
)

func TestContainerPruneProgress(t *testing.T) {
	var closed bool
	client, err := New(WithAPIVersion("1.56"), WithMockClient(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, req.URL.Query().Get("stream"), "true")
		assert.Equal(t, req.URL.Query().Get("filters"), `{"label":{"test=true":true}}`)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/jsonl"}},
			Body: &pruneTestBody{
				Reader: strings.NewReader("{\"id\":\"removed\",\"status\":\"deleted\"}\n{\"aux\":{\"ContainersDeleted\":[\"removed\"],\"SpaceReclaimed\":42}}\n"),
				closed: &closed,
			},
		}, nil
	}))
	assert.NilError(t, err)
	var updates []PruneProgress
	result, err := client.ContainerPrune(t.Context(), ContainerPruneOptions{
		Filters: make(Filters).Add("label", "test=true"),
		OnProgress: func(p PruneProgress) error {
			updates = append(updates, p)
			return nil
		},
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, updates, []PruneProgress{{ID: "removed", Action: "deleted"}})
	assert.DeepEqual(t, result.Report, container.PruneReport{ContainersDeleted: []string{"removed"}, SpaceReclaimed: 42})
	assert.Assert(t, closed)
}

type pruneTestBody struct {
	io.Reader
	closed *bool
}

func (b *pruneTestBody) Close() error {
	*b.closed = true
	return nil
}

func TestPruneProgressRequiresVersion(t *testing.T) {
	client, err := New(WithAPIVersion("1.55"), WithMockClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("prune must not run when progress is unsupported")
		return nil, nil
	}))
	assert.NilError(t, err)
	_, err = client.ContainerPrune(t.Context(), ContainerPruneOptions{OnProgress: func(PruneProgress) error { return nil }})
	assert.ErrorContains(t, err, "requires API version 1.56")
}

func TestPruneProgressFallbackWithoutRetry(t *testing.T) {
	requests := 0
	client, err := New(WithAPIVersion("1.56"), WithMockClient(func(req *http.Request) (*http.Response, error) {
		requests++
		assert.Equal(t, req.URL.Query().Get("stream"), "true")
		return mockJSONResponse(http.StatusOK, nil, container.PruneReport{
			ContainersDeleted: []string{"removed"},
			SpaceReclaimed:    42,
		})(req)
	}))
	assert.NilError(t, err)
	result, err := client.ContainerPrune(t.Context(), ContainerPruneOptions{OnProgress: func(PruneProgress) error {
		t.Fatal("legacy response must not produce progress messages")
		return nil
	}})
	assert.NilError(t, err)
	assert.Equal(t, requests, 1)
	assert.DeepEqual(t, result.Report, container.PruneReport{ContainersDeleted: []string{"removed"}, SpaceReclaimed: 42})
}

func TestDecodePruneStreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name, stream, errorText string
		expected                error
	}{
		{name: "empty", expected: io.ErrUnexpectedEOF},
		{name: "missing final report", stream: `{"id":"removed","status":"deleted"}`, expected: io.ErrUnexpectedEOF},
		{name: "daemon error", stream: `{"errorDetail":{"message":"prune failed"}}`, errorText: "prune failed"},
		{name: "invalid report", stream: `{"aux":{"SpaceReclaimed":"invalid"}}`, errorText: "cannot unmarshal"},
		{name: "invalid JSON", stream: `{`, expected: io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var report container.PruneReport
			err := decodePruneStream(strings.NewReader(tc.stream), func(PruneProgress) error { return nil }, &report)
			if tc.expected != nil {
				assert.ErrorIs(t, err, tc.expected)
			} else {
				assert.ErrorContains(t, err, tc.errorText)
			}
		})
	}
}

func TestPruneProgressCallbackErrorClosesResponse(t *testing.T) {
	var closed bool
	client, err := New(WithAPIVersion("1.56"), WithMockClient(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/jsonl"}},
			Body: &pruneTestBody{
				Reader: strings.NewReader(`{"id":"removed","status":"deleted"}`),
				closed: &closed,
			},
		}, nil
	}))
	assert.NilError(t, err)
	expected := errors.New("output failed")
	_, err = client.ContainerPrune(t.Context(), ContainerPruneOptions{OnProgress: func(PruneProgress) error { return expected }})
	assert.ErrorIs(t, err, expected)
	assert.Assert(t, closed)
}

func TestPruneProgressCallbackErrorCancelsLiveStream(t *testing.T) {
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/jsonl")
		_, _ = io.WriteString(w, "{\"id\":\"removed\",\"status\":\"deleted\"}\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	httpClient := server.Client()
	httpClient.Timeout = 5 * time.Second
	client, err := New(WithHost(server.URL), WithAPIVersion("1.56"), WithHTTPClient(httpClient))
	assert.NilError(t, err)
	expected := errors.New("output failed")
	_, err = client.ContainerPrune(t.Context(), ContainerPruneOptions{OnProgress: func(PruneProgress) error { return expected }})
	assert.ErrorIs(t, err, expected)
	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("daemon request was not canceled")
	}
}

func TestDecodePruneStreamEmptyReport(t *testing.T) {
	var report container.PruneReport
	err := decodePruneStream(strings.NewReader(`{"aux":{"ContainersDeleted":[],"SpaceReclaimed":0}}`), func(PruneProgress) error {
		t.Fatal("empty prune should have no progress")
		return nil
	}, &report)
	assert.NilError(t, err)
	assert.Equal(t, report.SpaceReclaimed, uint64(0))
}
