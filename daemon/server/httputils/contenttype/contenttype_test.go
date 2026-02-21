package contenttype_test

import (
	"net/http"
	"testing"

	"github.com/moby/moby/v2/daemon/server/httputils/contenttype"
)

func TestMatchAcceptStrict(t *testing.T) {
	tests := []struct {
		doc      string
		accept   string
		offers   []string
		expected string
	}{
		{
			doc:      "no Accept header",
			offers:   []string{"application/json"},
			expected: "",
		},
		{
			doc:      "no explicit match",
			accept:   "application/xml",
			offers:   []string{"application/json"},
			expected: "",
		},
		{
			doc:      "wildcard ignored",
			accept:   "*/*",
			offers:   []string{"application/json"},
			expected: "",
		},
		{
			doc:      "type wildcard ignored",
			accept:   "application/*",
			offers:   []string{"application/json"},
			expected: "",
		},
		{
			doc:      "exact match selected",
			accept:   "application/json",
			offers:   []string{"application/json"},
			expected: "application/json",
		},
		{
			doc:      "q=0 ignored",
			accept:   "application/json;q=0",
			offers:   []string{"application/json"},
			expected: "",
		},
		{
			doc:      "highest q wins",
			accept:   "application/json;q=0.5, application/xml;q=0.9",
			offers:   []string{"application/json", "application/xml"},
			expected: "application/xml",
		},
		{
			doc:      "tie q earlier offer wins",
			accept:   "application/json;q=0.8, application/xml;q=0.8",
			offers:   []string{"application/xml", "application/json"},
			expected: "application/xml",
		},
		{
			doc:      "duplicate media types highest q applies",
			accept:   "application/json;q=0.3, application/json;q=0.9",
			offers:   []string{"application/json"},
			expected: "application/json",
		},
		{
			doc:      "multiple accept entries first matching offer wins by q",
			accept:   "application/xml;q=0.7, application/json;q=0.6",
			offers:   []string{"application/json", "application/xml"},
			expected: "application/xml",
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()

			h := make(http.Header)
			if tc.accept != "" {
				h.Set("Accept", tc.accept)
			}

			got := contenttype.MatchAcceptStrict(h, tc.offers)
			if got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

// Code below is:
//
// Copyright 2013 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

var negotiateContentTypeTests = []struct {
	s            string
	offers       []string
	defaultOffer string
	expect       string
}{
	{"text/html, */*;q=0", []string{"x/y"}, "", ""},
	{"text/html, */*", []string{"x/y"}, "", "x/y"},
	{"text/html, image/png", []string{"text/html", "image/png"}, "", "text/html"},
	{"text/html, image/png", []string{"image/png", "text/html"}, "", "image/png"},
	{"text/html, image/png; q=0.5", []string{"image/png"}, "", "image/png"},
	{"text/html, image/png; q=0.5", []string{"text/html"}, "", "text/html"},
	{"text/html, image/png; q=0.5", []string{"foo/bar"}, "", ""},
	{"text/html, image/png; q=0.5", []string{"image/png", "text/html"}, "", "text/html"},
	{"text/html, image/png; q=0.5", []string{"text/html", "image/png"}, "", "text/html"},
	{"text/html;q=0.5, image/png", []string{"image/png"}, "", "image/png"},
	{"text/html;q=0.5, image/png", []string{"text/html"}, "", "text/html"},
	{"text/html;q=0.5, image/png", []string{"image/png", "text/html"}, "", "image/png"},
	{"text/html;q=0.5, image/png", []string{"text/html", "image/png"}, "", "image/png"},
	{"image/png, image/*;q=0.5", []string{"image/jpg", "image/png"}, "", "image/png"},
	{"image/png, image/*;q=0.5", []string{"image/jpg"}, "", "image/jpg"},
	{"image/png, image/*;q=0.5", []string{"image/jpg", "image/gif"}, "", "image/jpg"},
	{"image/png, image/*", []string{"image/jpg", "image/gif"}, "", "image/jpg"},
	{"image/png, image/*", []string{"image/gif", "image/jpg"}, "", "image/gif"},
	{"image/png, image/*", []string{"image/gif", "image/png"}, "", "image/png"},
	{"image/png, image/*", []string{"image/png", "image/gif"}, "", "image/png"},
}

func TestNegotiateContentType(t *testing.T) {
	for _, tt := range negotiateContentTypeTests {
		h := http.Header{"Accept": {tt.s}}
		actual := contenttype.Negotiate(h, tt.offers, tt.defaultOffer)
		if actual != tt.expect {
			t.Errorf("NegotiateContentType(%q, %#v, %q)=%q, want %q", tt.s, tt.offers, tt.defaultOffer, actual, tt.expect)
		}
	}
}
