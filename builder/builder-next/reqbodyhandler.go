package buildkit

import (
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/moby/buildkit/identity"
	"github.com/pkg/errors"
)

const urlPrefix = "build-context-"

type reqBodyHandler struct {
	mu sync.Mutex
	rt http.RoundTripper

	requests map[string]io.ReadCloser
}

func newReqBodyHandler(rt http.RoundTripper) *reqBodyHandler {
	return &reqBodyHandler{
		rt:       rt,
		requests: map[string]io.ReadCloser{},
	}
}

func (h *reqBodyHandler) newRequest(rc io.ReadCloser) (string, func()) {
	id := identity.NewID()
	h.mu.Lock()
	h.requests[id] = rc
	h.mu.Unlock()
	return "http://" + urlPrefix + id, func() {
		h.mu.Lock()
		delete(h.requests, id)
		h.mu.Unlock()
	}
}

func (h *reqBodyHandler) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	if strings.HasPrefix(host, urlPrefix) {
		if req.Method != "GET" {
			return nil, errors.Errorf("invalid request")
		}
		id := strings.TrimPrefix(host, urlPrefix)
		h.mu.Lock()
		rc, ok := h.requests[id]
		delete(h.requests, id)
		h.mu.Unlock()

		if !ok {
			return nil, errors.Errorf("context not found")
		}

		resp := &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			Body:          rc,
			ContentLength: -1,
		}

		return resp, nil
	}
	return h.rt.RoundTrip(req)
}
