package streams

import (
	"context"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/streams"
)

func (r *streamRouter) createStream(ctx context.Context, w http.ResponseWriter, req *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(req); err != nil {
		return err
	}

	var spec streams.Spec
	if err := httputils.ReadJSON(req, &spec); err != nil {
		return err
	}

	stream, err := r.backend.Create(ctx, req.Form.Get("id"), spec)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusCreated, stream)
}

func (r *streamRouter) getStream(ctx context.Context, w http.ResponseWriter, req *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(req); err != nil {
		return err
	}

	stream, err := r.backend.Get(ctx, vars["id"])
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, stream)
}

func (r *streamRouter) deleteStream(ctx context.Context, w http.ResponseWriter, req *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(req); err != nil {
		return err
	}

	if err := r.backend.Delete(ctx, vars["id"]); err != nil {
		return err
	}
	return nil
}
