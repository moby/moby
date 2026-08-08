package echov1

import (
	"context"

	"github.com/moby/moby/v2/internal/extensions"
)

// EchoServer is the provider interface for the echo test point.
type EchoServer interface {
	// Echo returns the request message, or an error when it is empty -- enough
	// to exercise both the success and veto paths over the wire.
	Echo(ctx context.Context, req *EchoRequest) (*EchoResponse, error)
}

// EchoRequest is the echo request.
type EchoRequest struct {
	Message string `pb:"1"`
}

// EchoResponse is the echo response.
type EchoResponse struct {
	Message string `pb:"1"`
}

// Point is the echo test point. Its id is a fixed string, distinct from any real
// point, so the launcher test drives the framework without touching one.
var Point = extensions.DefinePoint[EchoServer]("moby.extensions.internal.launcher.echo.v1")
