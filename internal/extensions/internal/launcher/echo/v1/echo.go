//go:generate go run github.com/moby/moby/v2/internal/extensions/cmd/mobyextgen

// Package echov1 is the minimal extension point used by launcher tests.
package echov1

import (
	"context"

	"github.com/moby/moby/v2/internal/extensions"
)

// EchoServer is the provider interface for the echo test point.
type EchoServer interface {
	// Echo returns the request message or an error when it is empty.
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

// Point is the echo test point used by launcher tests.
//
//mobyextgen:service=Echo
var Point = extensions.DefinePoint[EchoServer]("moby.extensions.internal.launcher.echo.v1")
