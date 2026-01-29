// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-openapi/strfmt"
)

// ClientRequestWriterFunc converts a function to a request writer interface
type ClientRequestWriterFunc func(ClientRequest, strfmt.Registry) error

// WriteToRequest adds data to the request
func (fn ClientRequestWriterFunc) WriteToRequest(req ClientRequest, reg strfmt.Registry) error {
	return fn(req, reg)
}

// ClientRequestWriter is an interface for things that know how to write to a request
type ClientRequestWriter interface {
	WriteToRequest(ClientRequest, strfmt.Registry) error
}

// ClientRequest is an interface for things that know how to
// add information to a swagger client request.
type ClientRequest interface { //nolint:interfacebloat // a swagger-capable request is quite rich, hence the many getter/setters
	SetHeaderParam(string, ...string) error

	GetHeaderParams() http.Header

	SetQueryParam(string, ...string) error

	SetFormParam(string, ...string) error

	SetPathParam(string, string) error

	GetQueryParams() url.Values

	SetFileParam(string, ...NamedReadCloser) error

	SetBodyParam(any) error

	SetTimeout(time.Duration) error

	GetMethod() string

	GetPath() string

	GetBody() []byte

	GetBodyParam() any

	GetFileParam() map[string][]NamedReadCloser
}

// NamedReadCloser represents a named ReadCloser interface
type NamedReadCloser interface {
	io.ReadCloser
	Name() string
}

// NamedReader creates a NamedReadCloser for use as file upload
func NamedReader(name string, rdr io.Reader) NamedReadCloser {
	rc, ok := rdr.(io.ReadCloser)
	if !ok {
		rc = io.NopCloser(rdr)
	}
	return &namedReadCloser{
		name: name,
		cr:   rc,
	}
}

type namedReadCloser struct {
	name string
	cr   io.ReadCloser
}

func (n *namedReadCloser) Close() error {
	return n.cr.Close()
}
func (n *namedReadCloser) Read(p []byte) (int, error) {
	return n.cr.Read(p)
}
func (n *namedReadCloser) Name() string {
	return n.name
}

type TestClientRequest struct {
	Headers http.Header
	Body    any
}

func (t *TestClientRequest) SetHeaderParam(name string, values ...string) error {
	if t.Headers == nil {
		t.Headers = make(http.Header)
	}
	t.Headers.Set(name, values[0])
	return nil
}

func (t *TestClientRequest) SetQueryParam(_ string, _ ...string) error { return nil }

func (t *TestClientRequest) SetFormParam(_ string, _ ...string) error { return nil }

func (t *TestClientRequest) SetPathParam(_ string, _ string) error { return nil }

func (t *TestClientRequest) SetFileParam(_ string, _ ...NamedReadCloser) error { return nil }

func (t *TestClientRequest) SetBodyParam(body any) error {
	t.Body = body
	return nil
}

func (t *TestClientRequest) SetTimeout(time.Duration) error {
	return nil
}

func (t *TestClientRequest) GetQueryParams() url.Values { return nil }

func (t *TestClientRequest) GetMethod() string { return "" }

func (t *TestClientRequest) GetPath() string { return "" }

func (t *TestClientRequest) GetBody() []byte { return nil }

func (t *TestClientRequest) GetBodyParam() any {
	return t.Body
}

func (t *TestClientRequest) GetFileParam() map[string][]NamedReadCloser {
	return nil
}

func (t *TestClientRequest) GetHeaderParams() http.Header {
	return t.Headers
}
