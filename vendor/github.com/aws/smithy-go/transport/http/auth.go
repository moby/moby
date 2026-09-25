package http

import (
	"context"

	smithy "github.com/aws/smithy-go"
	"github.com/aws/smithy-go/auth"
	"github.com/aws/smithy-go/eventstream"
)

// AuthScheme defines an HTTP authentication scheme.
type AuthScheme interface {
	SchemeID() string
	IdentityResolver(auth.IdentityResolverOptions) auth.IdentityResolver
	Signer() Signer
}

// Signer defines the interface through which HTTP requests are supplemented
// with an Identity.
type Signer interface {
	SignRequest(context.Context, *Request, auth.Identity, smithy.Properties) error
}

// EventStreamSigner is an optional interface that a [Signer] can implement to
// support signing of event stream messages. If the resolved auth scheme's
// signer implements this interface, the event stream middleware will use it to
// wrap the outbound message stream with a signing layer.
type EventStreamSigner interface {
	NewMessageSigner(ctx context.Context, r *Request, identity auth.Identity, props smithy.Properties) (eventstream.MessageSigner, error)
}
