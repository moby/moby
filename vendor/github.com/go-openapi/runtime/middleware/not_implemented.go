// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"

	"github.com/go-openapi/runtime"
)

type errorResp struct {
	code     int
	response any
	headers  http.Header
}

func (e *errorResp) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {
	for k, v := range e.headers {
		for _, val := range v {
			rw.Header().Add(k, val)
		}
	}
	if e.code > 0 {
		rw.WriteHeader(e.code)
	} else {
		rw.WriteHeader(http.StatusInternalServerError)
	}
	if err := producer.Produce(rw, e.response); err != nil {
		Logger.Printf("failed to write error response: %v", err)
	}
}

// NotImplemented the error response when the response is not implemented
func NotImplemented(message string) Responder {
	return Error(http.StatusNotImplemented, message)
}

// Error creates a generic responder for returning errors, the data will be serialized
// with the matching producer for the request
func Error(code int, data any, headers ...http.Header) Responder {
	var hdr http.Header
	for _, h := range headers {
		for k, v := range h {
			if hdr == nil {
				hdr = make(http.Header)
			}
			hdr[k] = v
		}
	}
	return &errorResp{
		code:     code,
		response: data,
		headers:  hdr,
	}
}
