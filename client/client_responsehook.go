package client

import (
	"net/http"
)

type responseHookTransport struct {
	base  http.RoundTripper
	hooks []ResponseHook
}

func (t *responseHookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	for _, h := range t.hooks {
		h(resp)
	}

	return resp, nil
}
