// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"net/http"
)

// ClientOperation represents the context for a swagger operation to be submitted to the transport.
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
	// Deprecated: prefer [ContextualTransport.SubmitContext] to pass the request context explicitly.
	Context context.Context //nolint:containedctx // we precisely want this type to contain the request context
	Client  *http.Client
}

// A ClientTransport implementor knows how to submit Request objects to some destination.
type ClientTransport interface {
	// Submit the operation and return the deserialized response or an error.
	Submit(*ClientOperation) (any, error)
}

// ContextualTransport extends [ClientTransport] with an explicit
// context-aware submission method.
//
// Wrappers such as the OpenTelemetry transport type-assert to this
// interface so they can forward an explicit context to the underlying
// transport without setting the cached [ClientOperation.Context] field.
//
// In v2, SubmitContext will be folded into [ClientTransport] itself
// and the cached [ClientOperation.Context] field removed; this interface
// is the v0.x bridge.
type ContextualTransport interface {
	ClientTransport

	// SubmitContext submits the operation using ctx as the request context.
	SubmitContext(ctx context.Context, operation *ClientOperation) (any, error)
}
