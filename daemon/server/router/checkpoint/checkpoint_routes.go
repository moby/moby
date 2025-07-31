package checkpoint

import (
	"context"
	"net/http"

	"github.com/moby/moby/api/types/checkpoint"
	"github.com/moby/moby/v2/daemon/server/httputils"
)

func (cr *checkpointRouter) postContainerCheckpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var options checkpoint.CreateOptions
	if err := httputils.ReadJSON(r, &options); err != nil {
		return err
	}

	err := cr.backend.CheckpointCreate(vars["name"], options)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusCreated)
	return nil
}

func (cr *checkpointRouter) getContainerCheckpoints(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	checkpoints, err := cr.backend.CheckpointList(vars["name"], checkpoint.ListOptions{
		CheckpointDir: r.Form.Get("dir"),
	})
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, checkpoints)
}

func (cr *checkpointRouter) deleteContainerCheckpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	err := cr.backend.CheckpointDelete(vars["name"], checkpoint.DeleteOptions{
		CheckpointDir: r.Form.Get("dir"),
		CheckpointID:  vars["checkpoint"],
	})
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
