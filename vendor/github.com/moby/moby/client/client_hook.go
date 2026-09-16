package client

import (
	"net/http"
)

type hookTransport struct {
	base      http.RoundTripper
	respHooks []ResponseHook
}

func (t *hookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	for _, h := range t.respHooks {
		h(resp)
	}

	return resp, nil
}
