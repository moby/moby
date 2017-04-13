package runtime

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	"golang.org/x/net/context"
)

func (rr *runtimeRouter) getRuntimes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	runtimes, err := rr.backend.ListRuntimes()
	if err != nil {
		logrus.Errorf("Error getting runtimes: %v", err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, runtimes)
}

func (rr *runtimeRouter) postRuntimeDefault(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := rr.backend.SetDefaultRuntime(r.Form.Get("runtime")); err != nil {
		return err
	}

	w.WriteHeader(http.StatusCreated)
	return nil
}
