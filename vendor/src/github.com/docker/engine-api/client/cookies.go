package client

import (
	"net/http"

	"github.com/docker/engine-api/client/transport"
	"golang.org/x/net/context"
)

// Wrap the call, or not, with cookie handling.  An http.Client would handle
// this for us if we passed in a cookie jar when we initialized it, but we'd
// still have to do the work for httputil.ClientConn, so we don't set it for
// elsewhere http.Clients to avoid duplicating cookies in requests.
func (cli *Client) cookieMiddleware(doer func(context.Context, transport.Sender, *http.Request) (*http.Response, error)) func(context.Context, transport.Sender, *http.Request) (*http.Response, error) {
	var jar http.CookieJar

	for _, auther := range cli.authers {
		if c, ok := auther.(CookieJarGetter); ok {
			jar = c.GetCookieJar()
			if jar != nil {
				break
			}
		}
	}
	if jar == nil {
		return doer
	}
	return func(ctx context.Context, client transport.Sender, req *http.Request) (resp *http.Response, err error) {
		if cli.transport != nil {
			req.URL.Scheme = cli.transport.Scheme()
		}
		for _, cookie := range jar.Cookies(req.URL) {
			req.AddCookie(cookie)
		}
		resp, err = doer(ctx, client, req)
		if resp != nil {
			if cookies := resp.Cookies(); cookies != nil {
				jar.SetCookies(req.URL, cookies)
			}
		}
		return resp, err
	}
}
