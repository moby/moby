package debug // import "github.com/docker/docker/api/server/router/debug"

import (
	"context"
	"net/http"
	"net/http/pprof"
)

func handlePprof(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	pprof.Handler(vars["name"]).ServeHTTP(w, r)
	return nil
}
