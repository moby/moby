// Copyright 2015 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

// This file implements a http.RoundTripper that authenticates
// requests issued against api.github.com endpoint.

package httputil

import (
	"net/http"
	"net/url"
)

// AuthTransport is an implementation of http.RoundTripper that authenticates
// with the GitHub API.
//
// When both a token and client credentials are set, the latter is preferred.
type AuthTransport struct {
	UserAgent          string
	GithubToken        string
	GithubClientID     string
	GithubClientSecret string
	Base               http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqCopy *http.Request
	if t.UserAgent != "" {
		reqCopy = copyRequest(req)
		reqCopy.Header.Set("User-Agent", t.UserAgent)
	}
	if req.URL.Host == "api.github.com" && req.URL.Scheme == "https" {
		switch {
		case t.GithubClientID != "" && t.GithubClientSecret != "":
			if reqCopy == nil {
				reqCopy = copyRequest(req)
			}
			if reqCopy.URL.RawQuery == "" {
				reqCopy.URL.RawQuery = "client_id=" + t.GithubClientID + "&client_secret=" + t.GithubClientSecret
			} else {
				reqCopy.URL.RawQuery += "&client_id=" + t.GithubClientID + "&client_secret=" + t.GithubClientSecret
			}
		case t.GithubToken != "":
			if reqCopy == nil {
				reqCopy = copyRequest(req)
			}
			reqCopy.Header.Set("Authorization", "token "+t.GithubToken)
		}
	}
	if reqCopy != nil {
		return t.base().RoundTrip(reqCopy)
	}
	return t.base().RoundTrip(req)
}

// CancelRequest cancels an in-flight request by closing its connection.
func (t *AuthTransport) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(req *http.Request)
	}
	if cr, ok := t.base().(canceler); ok {
		cr.CancelRequest(req)
	}
}

func (t *AuthTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func copyRequest(req *http.Request) *http.Request {
	req2 := new(http.Request)
	*req2 = *req
	req2.URL = new(url.URL)
	*req2.URL = *req.URL
	req2.Header = make(http.Header, len(req.Header))
	for k, s := range req.Header {
		req2.Header[k] = append([]string(nil), s...)
	}
	return req2
}
