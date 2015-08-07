package server

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/utils"
)

func (s *Server) getVersion(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v := &types.Version{
		Version:    dockerversion.VERSION,
		APIVersion: api.Version,
		GitCommit:  dockerversion.GITCOMMIT,
		GoVersion:  runtime.Version(),
		Os:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		BuildTime:  dockerversion.BUILDTIME,
	}

	if version.GreaterThanOrEqualTo("1.19") {
		v.Experimental = utils.ExperimentalBuild()
	}

	if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
		v.KernelVersion = kernelVersion.String()
	}

	return writeJSON(w, http.StatusOK, v)
}

func (s *Server) getInfo(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	info, err := s.daemon.SystemInfo()
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, info)
}

func (s *Server) getEvents(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	var since int64 = -1
	if r.Form.Get("since") != "" {
		s, err := strconv.ParseInt(r.Form.Get("since"), 10, 64)
		if err != nil {
			return err
		}
		since = s
	}

	var until int64 = -1
	if r.Form.Get("until") != "" {
		u, err := strconv.ParseInt(r.Form.Get("until"), 10, 64)
		if err != nil {
			return err
		}
		until = u
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

	isFiltered := func(field string, filter []string) bool {
		if len(field) == 0 {
			return false
		}
		if len(filter) == 0 {
			return false
		}
		for _, v := range filter {
			if v == field {
				return false
			}
			if strings.Contains(field, ":") {
				image := strings.Split(field, ":")
				if image[0] == v {
					return false
				}
			}
		}
		return true
	}

	d := s.daemon
	es := d.EventsService
	w.Header().Set("Content-Type", "application/json")
	outStream := ioutils.NewWriteFlusher(w)
	outStream.Write(nil) // make sure response is sent immediately
	enc := json.NewEncoder(outStream)

	getContainerID := func(cn string) string {
		c, err := d.Get(cn)
		if err != nil {
			return ""
		}
		return c.ID
	}

	sendEvent := func(ev *jsonmessage.JSONMessage) error {
		//incoming container filter can be name,id or partial id, convert and replace as a full container id
		for i, cn := range ef["container"] {
			ef["container"][i] = getContainerID(cn)
		}

		if isFiltered(ev.Status, ef["event"]) || (isFiltered(ev.ID, ef["image"]) &&
			isFiltered(ev.From, ef["image"])) || isFiltered(ev.ID, ef["container"]) {
			return nil
		}

		return enc.Encode(ev)
	}

	current, l := es.Subscribe()
	if since == -1 {
		current = nil
	}
	defer es.Evict(l)
	for _, ev := range current {
		if ev.Time < since {
			continue
		}
		if err := sendEvent(ev); err != nil {
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
			if err := sendEvent(jev); err != nil {
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
