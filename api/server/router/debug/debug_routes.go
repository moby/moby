package debug // import "github.com/docker/docker/api/server/router/debug"

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/pools"
	"github.com/pkg/errors"
)

func handlePprof(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	pprof.Handler(vars["name"]).ServeHTTP(w, r)
	return nil
}

func handleDumpFunc(b Backend) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		dump, err := b.SupportDump(ctx)
		if err != nil {
			return err
		}

		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Content-Encoding", "gzip")

		out := ioutils.NewWriteFlusher(gzip.NewWriter(w))
		if _, err = pools.Copy(out, dump); err != nil {
			return errors.Wrap(err, "error copying support dump to client")
		}
		return nil
	}
}
