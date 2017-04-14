package session

import (
	"net/http"

	"golang.org/x/net/context"
)

func (sr *sessionRouter) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return sr.backend.HandleHTTPRequest(ctx, w, r)
}
