package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/errdefs"
	"github.com/sirupsen/logrus"

	"github.com/docker/stacks/pkg/types"
)

func (sr *stacksRouter) getStacks(_ context.Context, w http.ResponseWriter, _ *http.Request, _ map[string]string) error {
	stacks, err := sr.backend.ListStacks()
	if err != nil {
		logrus.Errorf("error getting stacks: %s", err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, stacks)
}

func (sr *stacksRouter) createStack(_ context.Context, w http.ResponseWriter, r *http.Request, _ map[string]string) error {
	var stackCreate types.StackCreate
	if err := json.NewDecoder(r.Body).Decode(&stackCreate); err != nil {
		if err == io.EOF {
			return errdefs.InvalidParameter(errors.New("got EOF while reading request body"))
		}
		return errdefs.InvalidParameter(err)
	}

	resp, err := sr.backend.CreateStack(stackCreate)
	if err != nil {
		logrus.Errorf("Error creating stack: %s", err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusCreated, resp)
}

func (sr *stacksRouter) getStack(_ context.Context, w http.ResponseWriter, _ *http.Request, vars map[string]string) error {
	stack, err := sr.backend.GetStack(vars["id"])
	if err != nil {
		logrus.Errorf("Error getting stack %s: %s", vars["id"], err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, stack)
}

func (sr *stacksRouter) removeStack(_ context.Context, w http.ResponseWriter, _ *http.Request, vars map[string]string) error {
	err := sr.backend.DeleteStack(vars["id"])
	if err != nil {
		logrus.Errorf("Error removing stack %s: %s", vars["id"], err)
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (sr *stacksRouter) updateStack(_ context.Context, _ http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var stackSpec types.StackSpec
	if err := json.NewDecoder(r.Body).Decode(&stackSpec); err != nil {
		if err == io.EOF {
			return errdefs.InvalidParameter(errors.New("got EOF while reading request body"))
		}
		return errdefs.InvalidParameter(err)
	}

	rawVersion := r.URL.Query().Get("version")
	version, err := strconv.ParseUint(rawVersion, 10, 64)
	if err != nil {
		err := fmt.Errorf("invalid stack version '%s': %v", rawVersion, err)
		return errdefs.InvalidParameter(err)
	}

	err = sr.backend.UpdateStack(vars["id"], stackSpec, version)
	if err != nil {
		logrus.Errorf("Error updating stack %s: %s", vars["id"], err)
		return err
	}

	return nil
}

func (sr *stacksRouter) parseComposeInput(_ context.Context, w http.ResponseWriter, r *http.Request, _ map[string]string) error {
	var input types.ComposeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		if err == io.EOF {
			return errdefs.InvalidParameter(errors.New("got EOF while reading request body"))
		}
		return errdefs.InvalidParameter(err)
	}

	resp, err := sr.backend.ParseComposeInput(input)
	if err != nil {
		logrus.Errorf("Error creating stack: %s", err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, resp)
}
