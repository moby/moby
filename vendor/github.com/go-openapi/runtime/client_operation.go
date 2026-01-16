// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"net/http"
)

// ClientOperation represents the context for a swagger operation to be submitted to the transport
type ClientOperation struct {
	ID                 string
	Method             string
	PathPattern        string
	ProducesMediaTypes []string
	ConsumesMediaTypes []string
	Schemes            []string
	AuthInfo           ClientAuthInfoWriter
	Params             ClientRequestWriter
	Reader             ClientResponseReader
	Context            context.Context //nolint:containedctx // we precisely want this type to contain the request context
	Client             *http.Client
}

// A ClientTransport implementor knows how to submit Request objects to some destination
type ClientTransport interface {
	// Submit(string, RequestWriter, ResponseReader, AuthInfoWriter) (interface{}, error)
	Submit(*ClientOperation) (any, error)
}
