// Package singlemsg is a mobyextgen test fixture exercising a single embedded
// message field (*Nested). It is parsed by the generator test as source, not
// compiled or run.
package singlemsg

import (
	"context"

	"github.com/moby/moby/v2/internal/extensions"
)

// Service is the provider interface.
type Service interface {
	Do(ctx context.Context, req *Request) (*Response, error)
}

// Request embeds a single message by pointer.
type Request struct {
	Nested *Nested `pb:"1"`
}

// Response is a scalar reply.
type Response struct {
	Ok bool `pb:"1"`
}

// Nested is the embedded message.
type Nested struct {
	Value string `pb:"1"`
}

// Point is the test point.
var Point = extensions.DefinePoint[Service]("test.singlemsg.v1")
