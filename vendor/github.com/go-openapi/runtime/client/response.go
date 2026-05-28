// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"io"
	"net/http"

	"github.com/go-openapi/runtime"
)

var _ runtime.ClientResponse = response{}

func newResponse(resp *http.Response) runtime.ClientResponse { return response{resp: resp} }

type response struct {
	resp *http.Response
}

func (r response) Code() int {
	return r.resp.StatusCode
}

func (r response) Message() string {
	return r.resp.Status
}

func (r response) GetHeader(name string) string {
	return r.resp.Header.Get(name)
}

func (r response) GetHeaders(name string) []string {
	return r.resp.Header.Values(name)
}

func (r response) Body() io.ReadCloser {
	return r.resp.Body
}
