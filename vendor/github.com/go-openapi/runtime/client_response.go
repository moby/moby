// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// A ClientResponse represents a client response.
//
// This bridges between responses obtained from different transports
type ClientResponse interface {
	Code() int
	Message() string
	GetHeader(string) string
	GetHeaders(string) []string
	Body() io.ReadCloser
}

// A ClientResponseReaderFunc turns a function into a ClientResponseReader interface implementation
type ClientResponseReaderFunc func(ClientResponse, Consumer) (any, error)

// ReadResponse reads the response
func (read ClientResponseReaderFunc) ReadResponse(resp ClientResponse, consumer Consumer) (any, error) {
	return read(resp, consumer)
}

// A ClientResponseReader is an interface for things want to read a response.
// An application of this is to create structs from response values
type ClientResponseReader interface {
	ReadResponse(ClientResponse, Consumer) (any, error)
}

// APIError wraps an error model and captures the status code
type APIError struct {
	OperationName string
	Response      any
	Code          int
}

// NewAPIError creates a new API error
func NewAPIError(opName string, payload any, code int) *APIError {
	return &APIError{
		OperationName: opName,
		Response:      payload,
		Code:          code,
	}
}

// sanitizer ensures that single quotes are escaped
var sanitizer = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

func (o *APIError) Error() string {
	var resp []byte
	if err, ok := o.Response.(error); ok {
		resp = []byte("'" + sanitizer.Replace(err.Error()) + "'")
	} else {
		resp, _ = json.Marshal(o.Response)
	}

	return fmt.Sprintf("%s (status %d): %s", o.OperationName, o.Code, resp)
}

func (o *APIError) String() string {
	return o.Error()
}

// IsSuccess returns true when this API response returns a 2xx status code
func (o *APIError) IsSuccess() bool {
	const statusOK = 2
	return o.Code/100 == statusOK
}

// IsRedirect returns true when this API response returns a 3xx status code
func (o *APIError) IsRedirect() bool {
	const statusRedirect = 3
	return o.Code/100 == statusRedirect
}

// IsClientError returns true when this API response returns a 4xx status code
func (o *APIError) IsClientError() bool {
	const statusClientError = 4
	return o.Code/100 == statusClientError
}

// IsServerError returns true when this API response returns a 5xx status code
func (o *APIError) IsServerError() bool {
	const statusServerError = 5
	return o.Code/100 == statusServerError
}

// IsCode returns true when this API response returns a given status code
func (o *APIError) IsCode(code int) bool {
	return o.Code == code
}

// A ClientResponseStatus is a common interface implemented by all responses on the generated code
// You can use this to treat any client response based on status code
type ClientResponseStatus interface {
	IsSuccess() bool
	IsRedirect() bool
	IsClientError() bool
	IsServerError() bool
	IsCode(int) bool
}
