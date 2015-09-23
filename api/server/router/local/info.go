package local

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/utils"
	"golang.org/x/net/context"
)

func (s *router) getVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v := &types.Version{
		Version:    dockerversion.VERSION,
		APIVersion: api.Version,
		GitCommit:  dockerversion.GITCOMMIT,
		GoVersion:  runtime.Version(),
		Os:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		BuildTime:  dockerversion.BUILDTIME,
	}

	version := httputils.VersionFromContext(ctx)

	if version.GreaterThanOrEqualTo("1.19") {
		v.Experimental = utils.ExperimentalBuild()
	}

	if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
		v.KernelVersion = kernelVersion.String()
	}

	return httputils.WriteJSON(w, http.StatusOK, v)
}

func (s *router) getInfo(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	info, err := s.daemon.SystemInfo()
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, info)
}

func buildOutputEncoder(w http.ResponseWriter) *json.Encoder {
	w.Header().Set("Content-Type", "application/json")
	outStream := ioutils.NewWriteFlusher(w)
	// Write an empty chunk of data.
	// This is to ensure that the HTTP status code is sent immediately,
	// so that it will not block the receiver.
	outStream.Write(nil)
	return json.NewEncoder(outStream)
}

func (s *router) getEvents(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	since, err := httputils.Int64ValueOrDefault(r, "since", -1)
	if err != nil {
		return err
	}
	until, err := httputils.Int64ValueOrDefault(r, "until", -1)
	if err != nil {
		return err
	}

	timer := time.NewTimer(0)
	timer.Stop()
	if until > 0 {
		dur := time.Unix(until, 0).Sub(time.Now())
		timer = time.NewTimer(dur)
	}

	ef, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	enc := buildOutputEncoder(w)
	d := s.daemon
	es := d.EventsService
	current, l := es.Subscribe()
	defer es.Evict(l)

	eventFilter := d.GetEventFilter(ef)
	handleEvent := func(ev *jsonmessage.JSONMessage) error {
		if eventFilter.Include(ev) {
			if err := enc.Encode(ev); err != nil {
				return err
			}
		}
		return nil
	}

	if since == -1 {
		current = nil
	}
	for _, ev := range current {
		if ev.Time < since {
			continue
		}
		if err := handleEvent(ev); err != nil {
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
			if err := handleEvent(jev); err != nil {
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
