package client

import (
	"errors"
	"io"
	"net/http"
)

type hookTransport struct {
	base      http.RoundTripper
	reqHooks  []RequestHook
	respHooks []ResponseHook
}

func (t *hookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(t.reqHooks) > 0 {
		hookReq := req.Clone(req.Context())

		if hookReq.Body != nil {
			hookReq.Body = hookBody{}
		}
		if hookReq.GetBody != nil {
			hookReq.GetBody = func() (io.ReadCloser, error) {
				return hookBody{}, nil
			}
		}

		for _, h := range t.reqHooks {
			if err := h(hookReq); err != nil {
				return nil, err
			}
		}
		// Restore the actual request body after the hooks have run. The
		// modified request metadata is preserved on hookReq.
		hookReq.Body = req.Body
		hookReq.GetBody = req.GetBody
		req = hookReq
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if len(t.respHooks) > 0 {
		hookResp := *resp
		if hookResp.Body != nil {
			hookResp.Body = hookBody{}
		}

		for _, h := range t.respHooks {
			h(&hookResp)
		}
	}

	return resp, nil
}

type hookBody struct{}

func (hookBody) Read([]byte) (int, error) {
	return 0, errors.New("hooks must not read HTTP message body")
}

func (hookBody) Close() error { return nil }
