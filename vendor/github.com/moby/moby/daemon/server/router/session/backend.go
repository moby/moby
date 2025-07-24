package session

import (
	"context"
	"net/http"
)

// Backend abstracts an session receiver from an http request.
type Backend interface {
	HandleHTTPRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error
}
