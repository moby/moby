package server

import (
	"bufio"
	"bytes"
	"time"

	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"code.google.com/p/go.net/websocket"
	"github.com/gorilla/mux"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/networkdriver/bridge"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

var (
	activationLock = make(chan struct{})
)

type HttpServer struct {
	srv *http.Server
	l   net.Listener
}

func (s *HttpServer) Serve() error {
	return s.srv.Serve(s.l)
}
func (s *HttpServer) Close() error {
	return s.l.Close()
}

type HttpApiFunc func(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error

func hijackServer(w http.ResponseWriter) (io.ReadCloser, io.Writer, error) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	// Flush the options to make sure the client sets the raw mode
	conn.Write([]byte{})
	return conn, conn, nil
}

func closeStreams(streams ...interface{}) {
	for _, stream := range streams {
		if tcpc, ok := stream.(interface {
			CloseWrite() error
		}); ok {
			tcpc.CloseWrite()
		} else if closer, ok := stream.(io.Closer); ok {
			closer.Close()
		}
	}
}

// Check to make sure request's Content-Type is application/json
func checkForJson(r *http.Request) error {
	ct := r.Header.Get("Content-Type")

	// No Content-Type header is ok as long as there's no Body
	if ct == "" {
		if r.Body == nil || r.ContentLength == 0 {
			return nil
		}
	}

	// Otherwise it better be json
	if api.MatchesContentType(ct, "application/json") {
		return nil
	}
	return fmt.Errorf("Content-Type specified (%s) must be 'application/json'", ct)
}

//If we don't do this, POST method without Content-type (even with empty body) will fail
func parseForm(r *http.Request) error {
	if r == nil {
		return nil
	}
	if err := r.ParseForm(); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

func parseMultipartForm(r *http.Request) error {
	if err := r.ParseMultipartForm(4096); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

func httpError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	// FIXME: this is brittle and should not be necessary.
	// If we need to differentiate between different possible error types, we should
	// create appropriate error types with clearly defined meaning.
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "no such") {
		statusCode = http.StatusNotFound
	} else if strings.Contains(errStr, "bad parameter") {
		statusCode = http.StatusBadRequest
	} else if strings.Contains(errStr, "conflict") {
		statusCode = http.StatusConflict
	} else if strings.Contains(errStr, "impossible") {
		statusCode = http.StatusNotAcceptable
	} else if strings.Contains(errStr, "wrong login/password") {
		statusCode = http.StatusUnauthorized
	} else if strings.Contains(errStr, "hasn't been activated") {
		statusCode = http.StatusForbidden
	}

	if err != nil {
		logrus.Errorf("HTTP Error: statusCode=%d %v", statusCode, err)
		http.Error(w, err.Error(), statusCode)
	}
}

// writeJSONEnv writes the engine.Env values to the http response stream as a
// json encoded body.
func writeJSONEnv(w http.ResponseWriter, code int, v engine.Env) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return v.Encode(w)
}

// writeJSON writes the value v to the http response stream as json with standard
// json encoding.
func writeJSON(w http.ResponseWriter, code int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(v)
}

func streamJSON(job *engine.Job, w http.ResponseWriter, flush bool) {
	w.Header().Set("Content-Type", "application/json")
	if flush {
		job.Stdout.Add(utils.NewWriteFlusher(w))
	} else {
		job.Stdout.Add(w)
	}
}

func getDaemon(eng *engine.Engine) *daemon.Daemon {
	return eng.HackGetGlobalVar("httpapi.daemon").(*daemon.Daemon)
}

func postAuth(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var config *registry.AuthConfig
	err := json.NewDecoder(r.Body).Decode(&config)
	r.Body.Close()
	if err != nil {
		return err
	}
	d := getDaemon(eng)
	status, err := d.RegistryService.Auth(config)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, &types.AuthResponse{
		Status: status,
	})
}

func getVersion(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.Header().Set("Content-Type", "application/json")
	eng.ServeHTTP(w, r)
	return nil
}

func postContainersKill(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	err := parseForm(r)
	if err != nil {
		return err
	}

	var sig uint64
	name := vars["name"]

	// If we have a signal, look at it. Otherwise, do nothing
	if sigStr := vars["signal"]; sigStr != "" {
		// Check if we passed the signal as a number:
		// The largest legal signal is 31, so let's parse on 5 bits
		sig, err = strconv.ParseUint(sigStr, 10, 5)
		if err != nil {
			// The signal is not a number, treat it as a string (either like
			// "KILL" or like "SIGKILL")
			sig = uint64(signal.SignalMap[strings.TrimPrefix(sigStr, "SIG")])
		}

		if sig == 0 {
			return fmt.Errorf("Invalid signal: %s", sigStr)
		}
	}

	if err = getDaemon(eng).ContainerKill(name, sig); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainersPause(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	d := getDaemon(eng)
	cont, err := d.Get(name)
	if err != nil {
		return err
	}

	if err := cont.Pause(); err != nil {
		return fmt.Errorf("Cannot pause container %s: %s", name, err)
	}
	cont.LogEvent("pause")

	w.WriteHeader(http.StatusNoContent)

	return nil
}

func postContainersUnpause(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	d := getDaemon(eng)
	cont, err := d.Get(name)
	if err != nil {
		return err
	}

	if err := cont.Unpause(); err != nil {
		return fmt.Errorf("Cannot unpause container %s: %s", name, err)
	}
	cont.LogEvent("unpause")

	w.WriteHeader(http.StatusNoContent)

	return nil
}

func getContainersExport(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	d := getDaemon(eng)

	return d.ContainerExport(vars["name"], w)
}

func getImagesJSON(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	imagesConfig := graph.ImagesConfig{
		Filters: r.Form.Get("filters"),
		// FIXME this parameter could just be a match filter
		Filter: r.Form.Get("filter"),
		All:    toBool(r.Form.Get("all")),
	}

	images, err := getDaemon(eng).Repositories().Images(&imagesConfig)
	if err != nil {
		return err
	}

	if version.GreaterThanOrEqualTo("1.7") {
		return writeJSON(w, http.StatusOK, images)
	}

	legacyImages := []types.LegacyImage{}

	for _, image := range images {
		for _, repoTag := range image.RepoTags {
			repo, tag := parsers.ParseRepositoryTag(repoTag)
			legacyImage := types.LegacyImage{
				Repository:  repo,
				Tag:         tag,
				ID:          image.ID,
				Created:     image.Created,
				Size:        image.Size,
				VirtualSize: image.VirtualSize,
			}
			legacyImages = append(legacyImages, legacyImage)
		}
	}

	return writeJSON(w, http.StatusOK, legacyImages)
}

func getImagesViz(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version.GreaterThan("1.6") {
		w.WriteHeader(http.StatusNotFound)
		return fmt.Errorf("This is now implemented in the client.")
	}
	eng.ServeHTTP(w, r)
	return nil
}

func getInfo(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.Header().Set("Content-Type", "application/json")
	eng.ServeHTTP(w, r)
	return nil
}

func getEvents(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

	d := getDaemon(eng)
	es := d.EventsService
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(utils.NewWriteFlusher(w))

	getContainerId := func(cn string) string {
		c, err := d.Get(cn)
		if err != nil {
			return ""
		}
		return c.ID
	}

	sendEvent := func(ev *jsonmessage.JSONMessage) error {
		//incoming container filter can be name,id or partial id, convert and replace as a full container id
		for i, cn := range ef["container"] {
			ef["container"][i] = getContainerId(cn)
		}

		if isFiltered(ev.Status, ef["event"]) || isFiltered(ev.From, ef["image"]) ||
			isFiltered(ev.ID, ef["container"]) {
			return nil
		}

		return enc.Encode(ev)
	}

	current, l := es.Subscribe()
	defer es.Evict(l)
	for _, ev := range current {
		if ev.Time < since {
			continue
		}
		if err := sendEvent(ev); err != nil {
			return err
		}
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
		}
	}
}

func getImagesHistory(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	name := vars["name"]
	history, err := getDaemon(eng).Repositories().History(name)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, history)
}

func getContainersChanges(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	name := vars["name"]
	if name == "" {
		return fmt.Errorf("Container name cannot be empty")
	}

	d := getDaemon(eng)
	cont, err := d.Get(name)
	if err != nil {
		return err
	}

	changes, err := cont.Changes()
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, changes)
}

func getContainersTop(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version.LessThan("1.4") {
		return fmt.Errorf("top was improved a lot since 1.3, Please upgrade your docker client.")
	}

	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	if err := parseForm(r); err != nil {
		return err
	}

	procList, err := getDaemon(eng).ContainerTop(vars["name"], r.Form.Get("ps_args"))
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, procList)
}

func getContainersJSON(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var err error
	if err = parseForm(r); err != nil {
		return err
	}

	config := &daemon.ContainersConfig{
		All:     toBool(r.Form.Get("all")),
		Size:    toBool(r.Form.Get("size")),
		Since:   r.Form.Get("since"),
		Before:  r.Form.Get("before"),
		Filters: r.Form.Get("filters"),
	}

	if tmpLimit := r.Form.Get("limit"); tmpLimit != "" {
		config.Limit, err = strconv.Atoi(tmpLimit)
		if err != nil {
			return err
		}
	}

	containers, err := getDaemon(eng).Containers(config)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, containers)
}

func getContainersStats(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	d := getDaemon(eng)

	return d.ContainerStats(vars["name"], utils.NewWriteFlusher(w))
}

func getContainersLogs(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	// Validate args here, because we can't return not StatusOK after job.Run() call
	stdout, stderr := toBool(r.Form.Get("stdout")), toBool(r.Form.Get("stderr"))
	if !(stdout || stderr) {
		return fmt.Errorf("Bad parameters: you must choose at least one stream")
	}

	logsConfig := &daemon.ContainerLogsConfig{
		Follow:     toBool(r.Form.Get("follow")),
		Timestamps: toBool(r.Form.Get("timestamps")),
		Tail:       r.Form.Get("tail"),
		UseStdout:  stdout,
		UseStderr:  stderr,
		OutStream:  utils.NewWriteFlusher(w),
	}

	d := getDaemon(eng)
	if err := d.ContainerLogs(vars["name"], logsConfig); err != nil {
		fmt.Fprintf(w, "Error running logs job: %s\n", err)
	}

	return nil
}

func postImagesTag(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	job := eng.Job("tag", vars["name"], r.Form.Get("repo"), r.Form.Get("tag"))
	job.Setenv("force", r.Form.Get("force"))
	if err := job.Run(); err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func postCommit(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	var (
		config       engine.Env
		job          = eng.Job("commit", r.Form.Get("container"))
		stdoutBuffer = bytes.NewBuffer(nil)
	)

	if err := checkForJson(r); err != nil {
		return err
	}

	if err := config.Decode(r.Body); err != nil {
		logrus.Errorf("%s", err)
	}

	if r.FormValue("pause") == "" && version.GreaterThanOrEqualTo("1.13") {
		job.Setenv("pause", "1")
	} else {
		job.Setenv("pause", r.FormValue("pause"))
	}

	job.Setenv("repo", r.Form.Get("repo"))
	job.Setenv("tag", r.Form.Get("tag"))
	job.Setenv("author", r.Form.Get("author"))
	job.Setenv("comment", r.Form.Get("comment"))
	job.SetenvList("changes", r.Form["changes"])
	job.SetenvSubEnv("config", &config)

	job.Stdout.Add(stdoutBuffer)
	if err := job.Run(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, &types.ContainerCommitResponse{
		ID: engine.Tail(stdoutBuffer, 1),
	})
}

// Creates an image from Pull or from Import
func postImagesCreate(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	var (
		image = r.Form.Get("fromImage")
		repo  = r.Form.Get("repo")
		tag   = r.Form.Get("tag")
		job   *engine.Job
	)
	authEncoded := r.Header.Get("X-Registry-Auth")
	authConfig := &registry.AuthConfig{}
	if authEncoded != "" {
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			authConfig = &registry.AuthConfig{}
		}
	}
	if image != "" { //pull
		if tag == "" {
			image, tag = parsers.ParseRepositoryTag(image)
		}
		metaHeaders := map[string][]string{}
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-Meta-") {
				metaHeaders[k] = v
			}
		}
		job = eng.Job("pull", image, tag)
		job.SetenvBool("parallel", version.GreaterThan("1.3"))
		job.SetenvJson("metaHeaders", metaHeaders)
		job.SetenvJson("authConfig", authConfig)
	} else { //import
		if tag == "" {
			repo, tag = parsers.ParseRepositoryTag(repo)
		}
		job = eng.Job("import", r.Form.Get("fromSrc"), repo, tag)
		job.Stdin.Add(r.Body)
		job.SetenvList("changes", r.Form["changes"])
	}

	if version.GreaterThan("1.0") {
		job.SetenvBool("json", true)
		streamJSON(job, w, true)
	} else {
		job.Stdout.Add(utils.NewWriteFlusher(w))
	}
	if err := job.Run(); err != nil {
		if !job.Stdout.Used() {
			return err
		}
		sf := streamformatter.NewStreamFormatter(version.GreaterThan("1.0"))
		w.Write(sf.FormatError(err))
	}

	return nil
}

func getImagesSearch(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	var (
		config      *registry.AuthConfig
		authEncoded = r.Header.Get("X-Registry-Auth")
		headers     = map[string][]string{}
	)

	if authEncoded != "" {
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(&config); err != nil {
			// for a search it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			config = &registry.AuthConfig{}
		}
	}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			headers[k] = v
		}
	}
	d := getDaemon(eng)
	query, err := d.RegistryService.Search(r.Form.Get("term"), config, headers)
	if err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(query.Results)
}

func postImagesPush(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	metaHeaders := map[string][]string{}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}
	if err := parseForm(r); err != nil {
		return err
	}
	authConfig := &registry.AuthConfig{}

	authEncoded := r.Header.Get("X-Registry-Auth")
	if authEncoded != "" {
		// the new format is to handle the authConfig as a header
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// to increase compatibility to existing api it is defaulting to be empty
			authConfig = &registry.AuthConfig{}
		}
	} else {
		// the old format is supported for compatibility if there was no authConfig header
		if err := json.NewDecoder(r.Body).Decode(authConfig); err != nil {
			return fmt.Errorf("Bad parameters and missing X-Registry-Auth: %v", err)
		}
	}

	job := eng.Job("push", vars["name"])
	job.SetenvJson("metaHeaders", metaHeaders)
	job.SetenvJson("authConfig", authConfig)
	job.Setenv("tag", r.Form.Get("tag"))
	if version.GreaterThan("1.0") {
		job.SetenvBool("json", true)
		streamJSON(job, w, true)
	} else {
		job.Stdout.Add(utils.NewWriteFlusher(w))
	}

	if err := job.Run(); err != nil {
		if !job.Stdout.Used() {
			return err
		}
		sf := streamformatter.NewStreamFormatter(version.GreaterThan("1.0"))
		w.Write(sf.FormatError(err))
	}
	return nil
}

func getImagesGet(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}
	if version.GreaterThan("1.0") {
		w.Header().Set("Content-Type", "application/x-tar")
	}
	var job *engine.Job
	if name, ok := vars["name"]; ok {
		job = eng.Job("image_export", name)
	} else {
		job = eng.Job("image_export", r.Form["names"]...)
	}
	job.Stdout.Add(w)
	return job.Run()
}

func postImagesLoad(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	job := eng.Job("load")
	job.Stdin.Add(r.Body)
	job.Stdout.Add(w)
	return job.Run()
}

func postContainersCreate(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return nil
	}
	if err := checkForJson(r); err != nil {
		return err
	}
	var (
		job          = eng.Job("create", r.Form.Get("name"))
		outWarnings  []string
		stdoutBuffer = bytes.NewBuffer(nil)
		warnings     = bytes.NewBuffer(nil)
	)

	if err := job.DecodeEnv(r.Body); err != nil {
		return err
	}
	// Read container ID from the first line of stdout
	job.Stdout.Add(stdoutBuffer)
	// Read warnings from stderr
	job.Stderr.Add(warnings)
	if err := job.Run(); err != nil {
		return err
	}
	// Parse warnings from stderr
	scanner := bufio.NewScanner(warnings)
	for scanner.Scan() {
		outWarnings = append(outWarnings, scanner.Text())
	}
	return writeJSON(w, http.StatusCreated, &types.ContainerCreateResponse{
		ID:       engine.Tail(stdoutBuffer, 1),
		Warnings: outWarnings,
	})
}

func postContainersRestart(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	job := eng.Job("restart", vars["name"])
	job.Setenv("t", r.Form.Get("t"))
	if err := job.Run(); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainerRename(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	d := getDaemon(eng)
	name := vars["name"]
	newName := r.Form.Get("name")
	if err := d.ContainerRename(name, newName); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func deleteContainers(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	name := vars["name"]
	if name == "" {
		return fmt.Errorf("Container name cannot be empty")
	}

	d := getDaemon(eng)
	config := &daemon.ContainerRmConfig{
		ForceRemove:  toBool(r.Form.Get("force")),
		RemoveVolume: toBool(r.Form.Get("v")),
		RemoveLink:   toBool(r.Form.Get("link")),
	}

	if err := d.ContainerRm(name, config); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

func deleteImages(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	d := getDaemon(eng)
	name := vars["name"]
	force := toBool(r.Form.Get("force"))
	noprune := toBool(r.Form.Get("noprune"))

	list, err := d.ImageDelete(name, force, noprune)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, list)
}

func postContainersStart(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	var (
		name = vars["name"]
		job  = eng.Job("start", name)
	)

	// If contentLength is -1, we can assumed chunked encoding
	// or more technically that the length is unknown
	// http://golang.org/src/pkg/net/http/request.go#L139
	// net/http otherwise seems to swallow any headers related to chunked encoding
	// including r.TransferEncoding
	// allow a nil body for backwards compatibility
	if r.Body != nil && (r.ContentLength > 0 || r.ContentLength == -1) {
		if err := checkForJson(r); err != nil {
			return err
		}

		if err := job.DecodeEnv(r.Body); err != nil {
			return err
		}
	}

	if err := job.Run(); err != nil {
		if err.Error() == "Container already started" {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainersStop(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	d := getDaemon(eng)
	seconds, err := strconv.Atoi(r.Form.Get("t"))
	if err != nil {
		return err
	}

	if err := d.ContainerStop(vars["name"], seconds); err != nil {
		if err.Error() == "Container already stopped" {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
		return err
	}
	w.WriteHeader(http.StatusNoContent)

	return nil
}

func postContainersWait(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	name := vars["name"]
	d := getDaemon(eng)
	cont, err := d.Get(name)
	if err != nil {
		return err
	}

	status, _ := cont.WaitStop(-1 * time.Second)

	return writeJSON(w, http.StatusOK, &types.ContainerWaitResponse{
		StatusCode: status,
	})
}

func postContainersResize(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		return nil
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		return nil
	}

	d := getDaemon(eng)
	cont, err := d.Get(vars["name"])
	if err != nil {
		return err
	}

	if err := cont.Resize(height, width); err != nil {
		return err
	}

	return nil
}

func postContainersAttach(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	d := getDaemon(eng)

	cont, err := d.Get(vars["name"])
	if err != nil {
		return err
	}

	inStream, outStream, err := hijackServer(w)
	if err != nil {
		return err
	}
	defer closeStreams(inStream, outStream)

	var errStream io.Writer

	if _, ok := r.Header["Upgrade"]; ok {
		fmt.Fprintf(outStream, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
	} else {
		fmt.Fprintf(outStream, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	}

	if !cont.Config.Tty && version.GreaterThanOrEqualTo("1.6") {
		errStream = stdcopy.NewStdWriter(outStream, stdcopy.Stderr)
		outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
	} else {
		errStream = outStream
	}
	logs := toBool(r.Form.Get("logs"))
	stream := toBool(r.Form.Get("stream"))

	var stdin io.ReadCloser
	var stdout, stderr io.Writer

	if toBool(r.Form.Get("stdin")) {
		stdin = inStream
	}
	if toBool(r.Form.Get("stdout")) {
		stdout = outStream
	}
	if toBool(r.Form.Get("stderr")) {
		stderr = errStream
	}

	if err := cont.AttachWithLogs(stdin, stdout, stderr, logs, stream); err != nil {
		fmt.Fprintf(outStream, "Error attaching: %s\n", err)
	}
	return nil
}

func wsContainersAttach(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	d := getDaemon(eng)

	cont, err := d.Get(vars["name"])
	if err != nil {
		return err
	}

	h := websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		logs := r.Form.Get("logs") != ""
		stream := r.Form.Get("stream") != ""

		if err := cont.AttachWithLogs(ws, ws, ws, logs, stream); err != nil {
			logrus.Errorf("Error attaching websocket: %s", err)
		}
	})
	h.ServeHTTP(w, r)

	return nil
}

func getContainersByName(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	var job = eng.Job("container_inspect", vars["name"])
	if version.LessThan("1.12") {
		job.SetenvBool("raw", true)
	}
	streamJSON(job, w, false)
	return job.Run()
}

func getExecByID(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter 'id'")
	}

	d := getDaemon(eng)
	eConfig, err := d.ContainerExecInspect(vars["id"])
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, eConfig)
}

func getImagesByName(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	var job = eng.Job("image_inspect", vars["name"])
	if version.LessThan("1.12") {
		job.SetenvBool("raw", true)
	}
	streamJSON(job, w, false)
	return job.Run()
}

func postBuild(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version.LessThan("1.3") {
		return fmt.Errorf("Multipart upload for build is no longer supported. Please upgrade your docker client.")
	}
	var (
		authEncoded       = r.Header.Get("X-Registry-Auth")
		authConfig        = &registry.AuthConfig{}
		configFileEncoded = r.Header.Get("X-Registry-Config")
		configFile        = &registry.ConfigFile{}
		job               = eng.Job("build")
	)

	// This block can be removed when API versions prior to 1.9 are deprecated.
	// Both headers will be parsed and sent along to the daemon, but if a non-empty
	// ConfigFile is present, any value provided as an AuthConfig directly will
	// be overridden. See BuildFile::CmdFrom for details.
	if version.LessThan("1.9") && authEncoded != "" {
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			authConfig = &registry.AuthConfig{}
		}
	}

	if configFileEncoded != "" {
		configFileJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(configFileEncoded))
		if err := json.NewDecoder(configFileJson).Decode(configFile); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			configFile = &registry.ConfigFile{}
		}
	}

	if version.GreaterThanOrEqualTo("1.8") {
		job.SetenvBool("json", true)
		streamJSON(job, w, true)
	} else {
		job.Stdout.Add(utils.NewWriteFlusher(w))
	}

	if toBool(r.FormValue("forcerm")) && version.GreaterThanOrEqualTo("1.12") {
		job.Setenv("rm", "1")
	} else if r.FormValue("rm") == "" && version.GreaterThanOrEqualTo("1.12") {
		job.Setenv("rm", "1")
	} else {
		job.Setenv("rm", r.FormValue("rm"))
	}
	if toBool(r.FormValue("pull")) && version.GreaterThanOrEqualTo("1.16") {
		job.Setenv("pull", "1")
	}
	job.Stdin.Add(r.Body)
	job.Setenv("remote", r.FormValue("remote"))
	job.Setenv("dockerfile", r.FormValue("dockerfile"))
	job.Setenv("t", r.FormValue("t"))
	job.Setenv("q", r.FormValue("q"))
	job.Setenv("nocache", r.FormValue("nocache"))
	job.Setenv("forcerm", r.FormValue("forcerm"))
	job.SetenvJson("authConfig", authConfig)
	job.SetenvJson("configFile", configFile)
	job.Setenv("memswap", r.FormValue("memswap"))
	job.Setenv("memory", r.FormValue("memory"))
	job.Setenv("cpusetcpus", r.FormValue("cpusetcpus"))
	job.Setenv("cpushares", r.FormValue("cpushares"))

	// Job cancellation. Note: not all job types support this.
	if closeNotifier, ok := w.(http.CloseNotifier); ok {
		finished := make(chan struct{})
		defer close(finished)
		go func() {
			select {
			case <-finished:
			case <-closeNotifier.CloseNotify():
				logrus.Infof("Client disconnected, cancelling job: %s", job.Name)
				job.Cancel()
			}
		}()
	}

	if err := job.Run(); err != nil {
		if !job.Stdout.Used() {
			return err
		}
		sf := streamformatter.NewStreamFormatter(version.GreaterThanOrEqualTo("1.8"))
		w.Write(sf.FormatError(err))
	}
	return nil
}

func postContainersCopy(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	if err := checkForJson(r); err != nil {
		return err
	}

	cfg := types.CopyConfig{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		return err
	}

	if cfg.Resource == "" {
		return fmt.Errorf("Path cannot be empty")
	}

	res := cfg.Resource

	if res[0] == '/' {
		res = res[1:]
	}

	cont, err := getDaemon(eng).Get(vars["name"])
	if err != nil {
		logrus.Errorf("%v", err)
		if strings.Contains(strings.ToLower(err.Error()), "no such id") {
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
	}

	data, err := cont.Copy(res)
	if err != nil {
		logrus.Errorf("%v", err)
		if os.IsNotExist(err) {
			return fmt.Errorf("Could not find the file %s in container %s", cfg.Resource, vars["name"])
		}
		return err
	}
	defer data.Close()
	w.Header().Set("Content-Type", "application/x-tar")
	if _, err := io.Copy(w, data); err != nil {
		return err
	}

	return nil
}

func postContainerExecCreate(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return nil
	}
	var (
		name         = vars["name"]
		job          = eng.Job("execCreate", name)
		stdoutBuffer = bytes.NewBuffer(nil)
		outWarnings  []string
		warnings     = bytes.NewBuffer(nil)
	)

	if err := job.DecodeEnv(r.Body); err != nil {
		return err
	}

	job.Stdout.Add(stdoutBuffer)
	// Read warnings from stderr
	job.Stderr.Add(warnings)
	// Register an instance of Exec in container.
	if err := job.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up exec command in container %s: %s\n", name, err)
		return err
	}
	// Parse warnings from stderr
	scanner := bufio.NewScanner(warnings)
	for scanner.Scan() {
		outWarnings = append(outWarnings, scanner.Text())
	}

	return writeJSON(w, http.StatusCreated, &types.ContainerExecCreateResponse{
		ID:       engine.Tail(stdoutBuffer, 1),
		Warnings: outWarnings,
	})
}

// TODO(vishh): Refactor the code to avoid having to specify stream config as part of both create and start.
func postContainerExecStart(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return nil
	}
	var (
		name             = vars["name"]
		job              = eng.Job("execStart", name)
		errOut io.Writer = os.Stderr
	)

	if err := job.DecodeEnv(r.Body); err != nil {
		return err
	}
	if !job.GetenvBool("Detach") {
		// Setting up the streaming http interface.
		inStream, outStream, err := hijackServer(w)
		if err != nil {
			return err
		}
		defer closeStreams(inStream, outStream)

		var errStream io.Writer

		if _, ok := r.Header["Upgrade"]; ok {
			fmt.Fprintf(outStream, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
		} else {
			fmt.Fprintf(outStream, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
		}

		if !job.GetenvBool("Tty") && version.GreaterThanOrEqualTo("1.6") {
			errStream = stdcopy.NewStdWriter(outStream, stdcopy.Stderr)
			outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
		} else {
			errStream = outStream
		}
		job.Stdin.Add(inStream)
		job.Stdout.Add(outStream)
		job.Stderr.Set(errStream)
		errOut = outStream
	}
	// Now run the user process in container.
	job.SetCloseIO(false)
	if err := job.Run(); err != nil {
		fmt.Fprintf(errOut, "Error starting exec command in container %s: %s\n", name, err)
		return err
	}
	w.WriteHeader(http.StatusNoContent)

	return nil
}

func postContainerExecResize(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		return nil
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		return nil
	}

	d := getDaemon(eng)
	if err := d.ContainerExecResize(vars["name"], height, width); err != nil {
		return err
	}

	return nil
}

func optionsHandler(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.WriteHeader(http.StatusOK)
	return nil
}
func writeCorsHeaders(w http.ResponseWriter, r *http.Request, corsHeaders string) {
	logrus.Debugf("CORS header is enabled and set to: %s", corsHeaders)
	w.Header().Add("Access-Control-Allow-Origin", corsHeaders)
	w.Header().Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth")
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, OPTIONS")
}

func ping(eng *engine.Engine, version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	_, err := w.Write([]byte{'O', 'K'})
	return err
}

func makeHttpHandler(eng *engine.Engine, logging bool, localMethod string, localRoute string, handlerFunc HttpApiFunc, corsHeaders string, dockerVersion version.Version) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// log the request
		logrus.Debugf("Calling %s %s", localMethod, localRoute)

		if logging {
			logrus.Infof("%s %s", r.Method, r.RequestURI)
		}

		if strings.Contains(r.Header.Get("User-Agent"), "Docker-Client/") {
			userAgent := strings.Split(r.Header.Get("User-Agent"), "/")
			if len(userAgent) == 2 && !dockerVersion.Equal(version.Version(userAgent[1])) {
				logrus.Debugf("Warning: client and server don't have the same version (client: %s, server: %s)", userAgent[1], dockerVersion)
			}
		}
		version := version.Version(mux.Vars(r)["version"])
		if version == "" {
			version = api.APIVERSION
		}
		if corsHeaders != "" {
			writeCorsHeaders(w, r, corsHeaders)
		}

		if version.GreaterThan(api.APIVERSION) {
			http.Error(w, fmt.Errorf("client and server don't have same version (client API version: %s, server API version: %s)", version, api.APIVERSION).Error(), http.StatusNotFound)
			return
		}

		if err := handlerFunc(eng, version, w, r, mux.Vars(r)); err != nil {
			logrus.Errorf("Handler for %s %s returned error: %s", localMethod, localRoute, err)
			httpError(w, err)
		}
	}
}

// we keep enableCors just for legacy usage, need to be removed in the future
func createRouter(eng *engine.Engine, logging, enableCors bool, corsHeaders string, dockerVersion string) *mux.Router {
	r := mux.NewRouter()
	if os.Getenv("DEBUG") != "" {
		ProfilerSetup(r, "/debug/")
	}
	m := map[string]map[string]HttpApiFunc{
		"GET": {
			"/_ping":                          ping,
			"/events":                         getEvents,
			"/info":                           getInfo,
			"/version":                        getVersion,
			"/images/json":                    getImagesJSON,
			"/images/viz":                     getImagesViz,
			"/images/search":                  getImagesSearch,
			"/images/get":                     getImagesGet,
			"/images/{name:.*}/get":           getImagesGet,
			"/images/{name:.*}/history":       getImagesHistory,
			"/images/{name:.*}/json":          getImagesByName,
			"/containers/ps":                  getContainersJSON,
			"/containers/json":                getContainersJSON,
			"/containers/{name:.*}/export":    getContainersExport,
			"/containers/{name:.*}/changes":   getContainersChanges,
			"/containers/{name:.*}/json":      getContainersByName,
			"/containers/{name:.*}/top":       getContainersTop,
			"/containers/{name:.*}/logs":      getContainersLogs,
			"/containers/{name:.*}/stats":     getContainersStats,
			"/containers/{name:.*}/attach/ws": wsContainersAttach,
			"/exec/{id:.*}/json":              getExecByID,
		},
		"POST": {
			"/auth":                         postAuth,
			"/commit":                       postCommit,
			"/build":                        postBuild,
			"/images/create":                postImagesCreate,
			"/images/load":                  postImagesLoad,
			"/images/{name:.*}/push":        postImagesPush,
			"/images/{name:.*}/tag":         postImagesTag,
			"/containers/create":            postContainersCreate,
			"/containers/{name:.*}/kill":    postContainersKill,
			"/containers/{name:.*}/pause":   postContainersPause,
			"/containers/{name:.*}/unpause": postContainersUnpause,
			"/containers/{name:.*}/restart": postContainersRestart,
			"/containers/{name:.*}/start":   postContainersStart,
			"/containers/{name:.*}/stop":    postContainersStop,
			"/containers/{name:.*}/wait":    postContainersWait,
			"/containers/{name:.*}/resize":  postContainersResize,
			"/containers/{name:.*}/attach":  postContainersAttach,
			"/containers/{name:.*}/copy":    postContainersCopy,
			"/containers/{name:.*}/exec":    postContainerExecCreate,
			"/exec/{name:.*}/start":         postContainerExecStart,
			"/exec/{name:.*}/resize":        postContainerExecResize,
			"/containers/{name:.*}/rename":  postContainerRename,
		},
		"DELETE": {
			"/containers/{name:.*}": deleteContainers,
			"/images/{name:.*}":     deleteImages,
		},
		"OPTIONS": {
			"": optionsHandler,
		},
	}

	// If "api-cors-header" is not given, but "api-enable-cors" is true, we set cors to "*"
	// otherwise, all head values will be passed to HTTP handler
	if corsHeaders == "" && enableCors {
		corsHeaders = "*"
	}

	for method, routes := range m {
		for route, fct := range routes {
			logrus.Debugf("Registering %s, %s", method, route)
			// NOTE: scope issue, make sure the variables are local and won't be changed
			localRoute := route
			localFct := fct
			localMethod := method

			// build the handler function
			f := makeHttpHandler(eng, logging, localMethod, localRoute, localFct, corsHeaders, version.Version(dockerVersion))

			// add the new route
			if localRoute == "" {
				r.Methods(localMethod).HandlerFunc(f)
			} else {
				r.Path("/v{version:[0-9.]+}" + localRoute).Methods(localMethod).HandlerFunc(f)
				r.Path(localRoute).Methods(localMethod).HandlerFunc(f)
			}
		}
	}

	return r
}

// ServeRequest processes a single http request to the docker remote api.
// FIXME: refactor this to be part of Server and not require re-creating a new
// router each time. This requires first moving ListenAndServe into Server.
func ServeRequest(eng *engine.Engine, apiversion version.Version, w http.ResponseWriter, req *http.Request) {
	router := createRouter(eng, false, true, "", "")
	// Insert APIVERSION into the request as a convenience
	req.URL.Path = fmt.Sprintf("/v%s%s", apiversion, req.URL.Path)
	router.ServeHTTP(w, req)
}

func allocateDaemonPort(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	intPort, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	var hostIPs []net.IP
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		hostIPs = append(hostIPs, parsedIP)
	} else if hostIPs, err = net.LookupIP(host); err != nil {
		return fmt.Errorf("failed to lookup %s address in host specification", host)
	}

	for _, hostIP := range hostIPs {
		if _, err := bridge.RequestPort(hostIP, "tcp", intPort); err != nil {
			return fmt.Errorf("failed to allocate daemon listening port %d (err: %v)", intPort, err)
		}
	}
	return nil
}

type Server interface {
	Serve() error
	Close() error
}

// ServeApi loops through all of the protocols sent in to docker and spawns
// off a go routine to setup a serving http.Server for each.
func ServeApi(job *engine.Job) error {
	if len(job.Args) == 0 {
		return fmt.Errorf("usage: %s PROTO://ADDR [PROTO://ADDR ...]", job.Name)
	}
	var (
		protoAddrs = job.Args
		chErrors   = make(chan error, len(protoAddrs))
	)

	for _, protoAddr := range protoAddrs {
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		if len(protoAddrParts) != 2 {
			return fmt.Errorf("usage: %s PROTO://ADDR [PROTO://ADDR ...]", job.Name)
		}
		go func() {
			logrus.Infof("Listening for HTTP on %s (%s)", protoAddrParts[0], protoAddrParts[1])
			srv, err := NewServer(protoAddrParts[0], protoAddrParts[1], job)
			if err != nil {
				chErrors <- err
				return
			}
			job.Eng.OnShutdown(func() {
				if err := srv.Close(); err != nil {
					logrus.Error(err)
				}
			})
			if err = srv.Serve(); err != nil && strings.Contains(err.Error(), "use of closed network connection") {
				err = nil
			}
			chErrors <- err
		}()
	}

	for i := 0; i < len(protoAddrs); i++ {
		err := <-chErrors
		if err != nil {
			return err
		}
	}

	return nil
}

func toBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return !(s == "" || s == "0" || s == "no" || s == "false" || s == "none")
}
