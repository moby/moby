// Copyright 2013 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

package httputil

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type busterWriter struct {
	headerMap http.Header
	status    int
	io.Writer
}

func (bw *busterWriter) Header() http.Header {
	return bw.headerMap
}

func (bw *busterWriter) WriteHeader(status int) {
	bw.status = status
}

// CacheBusters maintains a cache of cache busting tokens for static resources served by Handler.
type CacheBusters struct {
	Handler http.Handler

	mu     sync.Mutex
	tokens map[string]string
}

func sanitizeTokenRune(r rune) rune {
	if r <= ' ' || r >= 127 {
		return -1
	}
	// Convert percent encoding reserved characters to '-'.
	if strings.ContainsRune("!#$&'()*+,/:;=?@[]", r) {
		return '-'
	}
	return r
}

// Get returns the cache busting token for path. If the token is not already
// cached, Get issues a HEAD request on handler and uses the response ETag and
// Last-Modified headers to compute a token.
func (cb *CacheBusters) Get(path string) string {
	cb.mu.Lock()
	if cb.tokens == nil {
		cb.tokens = make(map[string]string)
	}
	token, ok := cb.tokens[path]
	cb.mu.Unlock()
	if ok {
		return token
	}

	w := busterWriter{
		Writer:    ioutil.Discard,
		headerMap: make(http.Header),
	}
	r := &http.Request{URL: &url.URL{Path: path}, Method: "HEAD"}
	cb.Handler.ServeHTTP(&w, r)

	if w.status == 200 {
		token = w.headerMap.Get("Etag")
		if token == "" {
			token = w.headerMap.Get("Last-Modified")
		}
		token = strings.Trim(token, `" `)
		token = strings.Map(sanitizeTokenRune, token)
	}

	cb.mu.Lock()
	cb.tokens[path] = token
	cb.mu.Unlock()

	return token
}

// AppendQueryParam appends the token as a query parameter to path.
func (cb *CacheBusters) AppendQueryParam(path string, name string) string {
	token := cb.Get(path)
	if token == "" {
		return path
	}
	return path + "?" + name + "=" + token
}
