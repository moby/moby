package system

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	timetypes "github.com/docker/docker/api/types/time"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonmessage"
	"golang.org/x/net/context"
)

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

	return httputils.WriteJSON(w, http.StatusOK, info)
}

func (s *systemRouter) getVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	info := s.backend.SystemVersion()
	info.APIVersion = api.Version

	return httputils.WriteJSON(w, http.StatusOK, info)
}

func (s *systemRouter) getEvents(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	since, sinceNano, err := timetypes.ParseTimestamps(r.Form.Get("since"), -1)
	if err != nil {
		return err
	}
	until, untilNano, err := timetypes.ParseTimestamps(r.Form.Get("until"), -1)
	if err != nil {
		return err
	}

	timer := time.NewTimer(0)
	timer.Stop()
	if until > 0 || untilNano > 0 {
		dur := time.Unix(until, untilNano).Sub(time.Now())
		timer = time.NewTimer(dur)
	}

	ef, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")

	// This is to ensure that the HTTP status code is sent immediately,
	// so that it will not block the receiver.
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()

	enc := json.NewEncoder(output)

	buffered, l := s.backend.SubscribeToEvents(since, sinceNano, ef)
	defer s.backend.UnsubscribeFromEvents(l)

	for _, ev := range buffered {
		if err := enc.Encode(ev); err != nil {
			return err
		}
	}

	var closeNotify <-chan bool
	if closeNotifier, ok := w.(http.CloseNotifier); ok {
		closeNotify = closeNotifier.CloseNotify()
	}

	for {
		select {
		case ev := <-l:
			jev, ok := ev.(*jsonmessage.JSONMessage)
			if !ok {
				continue
			}
			if err := enc.Encode(jev); err != nil {
				return err
			}
		case <-timer.C:
			return nil
		case <-closeNotify:
			logrus.Debug("Client disconnected, stop sending events")
			return nil
		}
	}
}

func (s *systemRouter) postAuth(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var config *types.AuthConfig
	err := json.NewDecoder(r.Body).Decode(&config)
	r.Body.Close()
	if err != nil {
		return err
	}
	status, err := s.backend.AuthenticateToRegistry(config)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &types.AuthResponse{
		Status: status,
	})
}
