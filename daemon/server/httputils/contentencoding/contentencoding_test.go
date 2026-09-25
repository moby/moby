package contentencoding_test

import (
	"net/http"
	"testing"

	"github.com/moby/moby/v2/daemon/server/httputils/contentencoding"
)

func TestNegotiate(t *testing.T) {
	tests := []struct {
		doc            string
		acceptEncoding string
		offers         []string
		expected       string
		skip           string
	}{
		{
			doc:      "no Accept-Encoding header",
			offers:   []string{"gzip", "deflate"},
			expected: "identity",
		},
		{
			doc:            "gzip selected",
			acceptEncoding: "gzip",
			offers:         []string{"gzip", "deflate"},
			expected:       "gzip",
		},
		{
			doc:            "deflate selected",
			acceptEncoding: "deflate",
			offers:         []string{"gzip", "deflate"},
			expected:       "deflate",
		},
		{
			doc:            "highest q wins",
			acceptEncoding: "gzip;q=0.5, deflate;q=0.9",
			offers:         []string{"gzip", "deflate"},
			expected:       "deflate",
		},
		{
			doc:            "tie q earlier offer wins",
			acceptEncoding: "gzip;q=0.8, deflate;q=0.8",
			offers:         []string{"deflate", "gzip"},
			expected:       "deflate",
		},
		{
			doc:            "wildcard selects first offer",
			acceptEncoding: "*",
			offers:         []string{"gzip", "deflate"},
			expected:       "gzip",
		},
		{
			// RFC 9110 section 12.5.3 specifies that identity is acceptable by
			// default unless explicitly excluded.
			doc:            "rejected offer falls back to identity",
			acceptEncoding: "gzip;q=0",
			offers:         []string{"gzip", "deflate"},
			expected:       "identity",
			skip:           "FIXME: gddo does not correctly negotiate identity",
		},
		{
			doc:            "identity explicitly selected",
			acceptEncoding: "identity",
			offers:         []string{"gzip", "deflate"},
			expected:       "identity",
		},
		{
			doc:            "identity has higher quality",
			acceptEncoding: "gzip;q=0.5, identity;q=0.8",
			offers:         []string{"gzip"},
			expected:       "identity",
			skip:           "FIXME: gddo does not correctly negotiate identity",
		},
		{
			doc:            "all encodings rejected",
			acceptEncoding: "gzip;q=0, deflate;q=0, identity;q=0",
			offers:         []string{"gzip", "deflate"},
			expected:       "",
		},
		{
			doc:            "unsupported encoding falls back to identity",
			acceptEncoding: "unsupported",
			offers:         []string{"gzip", "deflate"},
			expected:       "identity",
		},
		{
			doc:            "unsupported encoding with identity rejected",
			acceptEncoding: "unsupported, identity;q=0",
			offers:         []string{"gzip", "deflate"},
			expected:       "",
			skip:           "FIXME: gddo does not correctly negotiate identity",
		},
		{
			// Content-coding names are case-insensitive per RFC 9110.
			doc:            "encoding is case-insensitive",
			acceptEncoding: "GZip",
			offers:         []string{"gzip", "deflate"},
			expected:       "gzip",
			skip:           "FIXME: gddo matches content-coding names case-sensitively",
		},
		{
			doc:            "offer is case-insensitive",
			acceptEncoding: "gzip",
			offers:         []string{"GZip", "deflate"},
			expected:       "GZip",
			skip:           "FIXME: gddo matches content-coding names case-sensitively",
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			if tc.skip != "" {
				t.Skip(tc.skip)
			}
			h := make(http.Header)
			if tc.acceptEncoding != "" {
				h.Set("Accept-Encoding", tc.acceptEncoding)
			}

			got := contentencoding.Negotiate(h, tc.offers)
			if got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}
