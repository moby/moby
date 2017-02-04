package system

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	timetypes "github.com/docker/docker/api/types/time"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/pkg/ioutils"
	pkgerrors "github.com/pkg/errors"
	"golang.org/x/net/context"
)

var acceptedEventFilters = map[string]bool{
	"config":    true,
	"container": true,
	"daemon":    true,
	"event":     true,
	"image":     true,
	"label":     true,
	"network":   true,
	"node":      true,
	"plugin":    true,
	"scope":     true,
	"secret":    true,
	"service":   true,
	"type":      true,
	"volume":    true,
}

func optionsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.WriteHeader(http.StatusOK)
	return nil
}

func pingHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	_, err := w.Write([]byte{'O', 'K'})
	return err
}

func (s *systemRouter) getInfo(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	info, err := s.backend.SystemInfo()
	if err != nil {
		return err
	}
	if s.cluster != nil {
		info.Swarm = s.cluster.Info()
	}

	if versions.LessThan(httputils.VersionFromContext(ctx), "1.25") {
		// TODO: handle this conversion in engine-api
		type oldInfo struct {
			*types.Info
			ExecutionDriver string
		}
		old := &oldInfo{
			Info:            info,
			ExecutionDriver: "<not supported>",
		}
		nameOnlySecurityOptions := []string{}
		kvSecOpts, err := types.DecodeSecurityOptions(old.SecurityOptions)
		if err != nil {
			return err
		}
		for _, s := range kvSecOpts {
			nameOnlySecurityOptions = append(nameOnlySecurityOptions, s.Name)
		}
		old.SecurityOptions = nameOnlySecurityOptions
		return httputils.WriteJSON(w, http.StatusOK, old)
	}
	return httputils.WriteJSON(w, http.StatusOK, info)
}

func (s *systemRouter) getVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	info := s.backend.SystemVersion()
	info.APIVersion = api.DefaultVersion

	return httputils.WriteJSON(w, http.StatusOK, info)
}

func (s *systemRouter) getDiskUsage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	du, err := s.backend.SystemDiskUsage(ctx)
	if err != nil {
		return err
	}
	builderSize, err := s.builder.DiskUsage()
	if err != nil {
		return pkgerrors.Wrap(err, "error getting build cache usage")
	}
	du.BuilderSize = builderSize

	return httputils.WriteJSON(w, http.StatusOK, du)
}

type invalidRequestError struct {
	Err error
}

func (e invalidRequestError) Error() string {
	return e.Err.Error()
}

func (e invalidRequestError) InvalidParameter() {}

func (s *systemRouter) getEvents(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	eventFilters, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		return err
	}
	if err := eventFilters.Validate(acceptedEventFilters); err != nil {
		return err
	}

	since, err := eventTime(r.Form.Get("since"))
	if err != nil {
		return err
	}
	until, err := eventTime(r.Form.Get("until"))
	if err != nil {
		return err
	}

	if !until.IsZero() && until.Before(since) {
		return invalidRequestError{fmt.Errorf("`since` time (%s) cannot be after `until` time (%s)", r.Form.Get("since"), r.Form.Get("until"))}
	}

	w.Header().Set("Content-Type", "application/json")
	output := ioutils.NewWriteFlusher(w)

	defer output.Close()
	output.Flush()

	enc := json.NewEncoder(output)

	return s.backend.Events(ctx, since, until, enc, eventFilters)
}

func (s *systemRouter) postAuth(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var config *types.AuthConfig
	err := json.NewDecoder(r.Body).Decode(&config)
	r.Body.Close()
	if err != nil {
		return err
	}
	status, token, err := s.backend.AuthenticateToRegistry(ctx, config)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &registry.AuthenticateOKBody{
		Status:        status,
		IdentityToken: token,
	})
}

func eventTime(formTime string) (time.Time, error) {
	t, tNano, err := timetypes.ParseTimestamps(formTime, -1)
	if err != nil {
		return time.Time{}, err
	}
	if t == -1 {
		return time.Time{}, nil
	}
	return time.Unix(t, tNano), nil
}
