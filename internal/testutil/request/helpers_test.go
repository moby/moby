package request_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
)

func TestReadJSONResponse(t *testing.T) {
	type someResponse struct {
		Hello string `json:"Hello"`
	}

	tests := []struct {
		name        string
		body        string
		contentType string
		expected    someResponse
		expErr      string
	}{
		{
			name:        "valid JSON",
			body:        `{"hello": "world"}`,
			contentType: "application/json",
			expected:    someResponse{Hello: "world"},
		},
		{
			name:        "valid JSON, utf-8",
			body:        `{"hello": "world"}`,
			contentType: "application/json; charset=utf-8",
			expected:    someResponse{Hello: "world"},
		},
		{
			name:        "malformed JSON",
			body:        `{"hello": "world"`,
			contentType: "application/json",
			expErr:      "unexpected EOF",
		},
		{
			name:        "non-JSON",
			body:        `<html><head><title>Page not found</title></head></html>`,
			contentType: "text/html",
			expErr:      "unexpected Content-Type: 'text/html'",
		},
		{
			name:   "nil response",
			expErr: "nil *http.Response",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v someResponse

			var resp *http.Response
			if tc.name != "nil response" {
				resp = &http.Response{
					Header: make(http.Header),
					Body:   io.NopCloser(bytes.NewBufferString(tc.body)),
				}
				resp.Header.Set("Content-Type", tc.contentType)
			}

			err := request.ReadJSONResponse(resp, &v)
			if tc.expErr != "" {
				assert.ErrorContains(t, err, tc.expErr)
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, tc.expected, v)
			}
		})
	}
}
