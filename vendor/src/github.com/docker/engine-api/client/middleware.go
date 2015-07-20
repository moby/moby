package client

import (
	"net/http"

	"github.com/docker/engine-api/client/authn"
	"github.com/docker/engine-api/client/transport"
	"golang.org/x/net/context"
)

func (cli *Client) doWithMiddlewares(d func(context.Context, transport.Sender, *http.Request) (*http.Response, error)) func(context.Context, transport.Sender, *http.Request) (*http.Response, error) {
	middlewares := []func(func(context.Context, transport.Sender, *http.Request) (*http.Response, error)) func(context.Context, transport.Sender, *http.Request) (*http.Response, error){
		cli.cookieMiddleware,
		authn.Middleware(cli.logger, cli.authers...),
	}
	for _, m := range middlewares {
		d = m(d)
	}
	return d
}
