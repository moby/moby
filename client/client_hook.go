package client

import (
	"errors"
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

	hookResp := *resp
	if hookResp.Body != nil {
		hookResp.Body = hookBody{}
	}

	for _, h := range t.respHooks {
		h(&hookResp)
	}

	return resp, nil
}

type hookBody struct{}

func (hookBody) Read([]byte) (int, error) {
	return 0, errors.New("hooks must not read HTTP message body")
}

func (hookBody) Close() error { return nil }
