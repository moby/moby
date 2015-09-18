package server

import (
	"fmt"
	"net/http"

	"github.com/docker/docker/context"
	"github.com/docker/docker/opts"
)

func (s *Server) getLabelList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	labels, err := s.daemon.Labels()
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, labels)
}

func (s *Server) postLabelsAdd(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	req, err := opts.ValidateLabel(vars["name"])
	if err != nil {
		return err
	}
	labels, err := s.daemon.LabelAdd(req)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, labels)
}

func (s *Server) deleteLabels(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if err := s.daemon.LabelRm(vars["name"]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
