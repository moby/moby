// Command exthook is the out-of-process extension used by launcher tests.
package main

import (
	"context"
	"errors"

	"github.com/moby/moby/v2/internal/extensions"
	echov1 "github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1"
	echopb "github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1/protogen"
	"github.com/moby/moby/v2/internal/extensions/sdk"
)

type echo struct{}

var extensionID = "org.example.exthook.v1"

func (echo) Echo(_ context.Context, req *echov1.EchoRequest) (*echov1.EchoResponse, error) {
	if req.Message == "" {
		return nil, errors.New("message must not be empty")
	}
	return &echov1.EchoResponse{Message: req.Message}, nil
}

func main() {
	ext := extensions.New(extensions.Declaration{
		ID:        extensions.ExtensionID(extensionID),
		Providers: []extensions.Provider{echov1.Point.Provide(echo{})},
	})
	sdk.Main(ext, echopb.ServerPoint)
}
