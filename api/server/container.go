package server

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

// @Title getContainersJSON
// @Description Retrieve the JSON representation of a list of containers
// @Param   version     path    string     false        "API version number"
// @Success 200 {array} types.Container
// @SubApi /containers
// @Router /containers/json [get]
func (s *Server) getContainersJSON(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	config := &daemon.ContainersConfig{
		All:     boolValue(r, "all"),
		Size:    boolValue(r, "size"),
		Since:   r.Form.Get("since"),
		Before:  r.Form.Get("before"),
		Filters: r.Form.Get("filters"),
	}

	if tmpLimit := r.Form.Get("limit"); tmpLimit != "" {
		limit, err := strconv.Atoi(tmpLimit)
		if err != nil {
			return err
		}
		config.Limit = limit
	}

	containers, err := s.daemon.Containers(config)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, containers)
}

// @Title getContainersStats
// @Description Retrieve the JSON representation of a container stats
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   stream      query   boolean    false        "Container ID or name"
// @Success 200 {array} types.Stats
// @SubApi /containers
// @Router /containers/:name/stats [get]
func (s *Server) getContainersStats(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	stream := boolValueOrDefault(r, "stream", true)
	var out io.Writer
	if !stream {
		w.Header().Set("Content-Type", "application/json")
		out = w
	} else {
		out = ioutils.NewWriteFlusher(w)
	}

	var closeNotifier <-chan bool
	if notifier, ok := w.(http.CloseNotifier); ok {
		closeNotifier = notifier.CloseNotify()
	}

	config := &daemon.ContainerStatsConfig{
		Stream:    stream,
		OutStream: out,
		Stop:      closeNotifier,
		Version:   versionFromContext(ctx),
	}

	return s.daemon.ContainerStats(vars["name"], config)
}

// @Title getContainersLogs
// @Description Retrieve the JSON representation of a container logs
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   stdout      query   boolean    false        "Display stdout messages"
// @Param   stderr      query   boolean    false        "Display stderr messages"
// @Param   since       query   integer    false        "Display logs since a given timestamp"
// @Param   follow      query   boolean    false        "Stream the logs to the output"
// @Param   timestamps  query   boolean    false        "Add timestamps to the logs or not"
// @Param   tail        query   boolean    false        "Retrieve N number of lines from the tail of the logs"
// @Success 200 {array} []byte
// @SubApi /containers
// @Router /containers/:name/logs [get]
func (s *Server) getContainersLogs(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	// Args are validated before the stream starts because when it starts we're
	// sending HTTP 200 by writing an empty chunk of data to tell the client that
	// daemon is going to stream. By sending this initial HTTP 200 we can't report
	// any error after the stream starts (i.e. container not found, wrong parameters)
	// with the appropriate status code.
	stdout, stderr := boolValue(r, "stdout"), boolValue(r, "stderr")
	if !(stdout || stderr) {
		return fmt.Errorf("Bad parameters: you must choose at least one stream")
	}

	var since time.Time
	if r.Form.Get("since") != "" {
		s, err := strconv.ParseInt(r.Form.Get("since"), 10, 64)
		if err != nil {
			return err
		}
		since = time.Unix(s, 0)
	}

	var closeNotifier <-chan bool
	if notifier, ok := w.(http.CloseNotifier); ok {
		closeNotifier = notifier.CloseNotify()
	}

	c, err := s.daemon.Get(vars["name"])
	if err != nil {
		return err
	}

	outStream := ioutils.NewWriteFlusher(w)
	// write an empty chunk of data (this is to ensure that the
	// HTTP Response is sent immediately, even if the container has
	// not yet produced any data)
	outStream.Write(nil)

	logsConfig := &daemon.ContainerLogsConfig{
		Follow:     boolValue(r, "follow"),
		Timestamps: boolValue(r, "timestamps"),
		Since:      since,
		Tail:       r.Form.Get("tail"),
		UseStdout:  stdout,
		UseStderr:  stderr,
		OutStream:  outStream,
		Stop:       closeNotifier,
	}

	if err := s.daemon.ContainerLogs(c, logsConfig); err != nil {
		// The client may be expecting all of the data we're sending to
		// be multiplexed, so send it through OutStream, which will
		// have been set up to handle that if needed.
		fmt.Fprintf(logsConfig.OutStream, "Error running logs job: %s\n", utils.GetErrorMessage(err))
	}

	return nil
}

// @Title getContainersExport
// @Description Export a container as a tarball
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Success 200 {array} []byte
// @SubApi /containers
// @Router /containers/:name/export [get]
func (s *Server) getContainersExport(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	return s.daemon.ContainerExport(vars["name"], w)
}

// @Title postContainersStart
// @Description Start a container that has previously been created
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   hostConfig  body    []byte     false        "Container host configuration"
// @Success 204
// @SubApi /containers
// @Router /containers/:name/start [post]
func (s *Server) postContainersStart(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	// If contentLength is -1, we can assumed chunked encoding
	// or more technically that the length is unknown
	// https://golang.org/src/pkg/net/http/request.go#L139
	// net/http otherwise seems to swallow any headers related to chunked encoding
	// including r.TransferEncoding
	// allow a nil body for backwards compatibility
	var hostConfig *runconfig.HostConfig
	if r.Body != nil && (r.ContentLength > 0 || r.ContentLength == -1) {
		if err := checkForJSON(r); err != nil {
			return err
		}

		c, err := runconfig.DecodeHostConfig(r.Body)
		if err != nil {
			return err
		}

		hostConfig = c
	}

	if err := s.daemon.ContainerStart(vars["name"], hostConfig); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// @Title postContainersStop
// @Description Stop a running container
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Success 204
// @SubApi /containers
// @Router /containers/:name/stop [post]
func (s *Server) postContainersStop(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	seconds, _ := strconv.Atoi(r.Form.Get("t"))

	if err := s.daemon.ContainerStop(vars["name"], seconds); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)

	return nil
}

// @Title postContainersKill
// @Description Kill a running container
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   signal      query   string     false        "Signal to kill the container"
// @Success 204
// @SubApi /containers
// @Router /containers/:name/kill [post]
func (s *Server) postContainersKill(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}

	var sig syscall.Signal
	name := vars["name"]

	// If we have a signal, look at it. Otherwise, do nothing
	if sigStr := r.Form.Get("signal"); sigStr != "" {
		var err error
		if sig, err = signal.ParseSignal(sigStr); err != nil {
			return err
		}
	}

	if err := s.daemon.ContainerKill(name, uint64(sig)); err != nil {
		theErr, isDerr := err.(errcode.ErrorCoder)
		isStopped := isDerr && theErr.ErrorCode() == derr.ErrorCodeNotRunning

		// Return error that's not caused because the container is stopped.
		// Return error if the container is not running and the api is >= 1.20
		// to keep backwards compatibility.
		version := versionFromContext(ctx)
		if version.GreaterThanOrEqualTo("1.20") || !isStopped {
			return fmt.Errorf("Cannot kill container %s: %v", name, err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// @Title postContainersRestart
// @Description Restart a running container
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   t           query   integer    false        "Timeout to wait until the container starts again"
// @Success 204
// @SubApi /containers
// @Router /containers/:name/restart [post]
func (s *Server) postContainersRestart(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	timeout, _ := strconv.Atoi(r.Form.Get("t"))

	if err := s.daemon.ContainerRestart(vars["name"], timeout); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

// @Title postContainersPause
// @Description Pause a running container
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Success 204
// @SubApi /containers
// @Router /containers/:name/pause [post]
func (s *Server) postContainersPause(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}

	if err := s.daemon.ContainerPause(vars["name"]); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

// @Title postContainersUnpause
// @Description Unpause a paused container
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Success 204
// @SubApi /containers
// @Router /containers/:name/unpause [post]
func (s *Server) postContainersUnpause(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}

	if err := s.daemon.ContainerUnpause(vars["name"]); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

// @Title postContainersWait
// @Description Wait for a container until it starts
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Success 200 {object} types.ContainerWaitResponse
// @SubApi /containers
// @Router /containers/:name/wait [post]
func (s *Server) postContainersWait(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	status, err := s.daemon.ContainerWait(vars["name"], -1*time.Second)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, &types.ContainerWaitResponse{
		StatusCode: status,
	})
}

// @Title getContainersChanges
// @Description Get a list of changes in the container filesystem since it was started
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Success 200 {object} archive.Changes
// @SubApi /containers
// @Router /containers/:name/changes [get]
func (s *Server) getContainersChanges(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	changes, err := s.daemon.ContainerChanges(vars["name"])
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, changes)
}

// @Title getContainersTop
// @Description Get a list of processes running inside the container and their status
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   ps_args     query   string     false        "Arguments to send to the top command inside the container"
// @Success 200 {object} types.ContainerProcessList
// @SubApi /containers
// @Router /containers/:name/top [get]
func (s *Server) getContainersTop(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	if err := parseForm(r); err != nil {
		return err
	}

	procList, err := s.daemon.ContainerTop(vars["name"], r.Form.Get("ps_args"))
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, procList)
}

// @Title postContainerRename
// @Description Rename a container
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   name        form    string     true         "New name for the container"
// @Success 204
// @SubApi /containers
// @Router /containers/:name/rename [post]
func (s *Server) postContainerRename(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	name := vars["name"]
	newName := r.Form.Get("name")
	if err := s.daemon.ContainerRename(name, newName); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// @Title postContainerCreate
// @Description Create a new container from an image
// @Param   version     path    string     false        "API version number"
// @Param   name        form    string     false        "Container name"
// @Param   hostConfig  body    []byte     false        "Container host configuration"
// @Success 201 {object} types.ContainerCreateResponse
// @SubApi /containers
// @Router /containers/:name/create [post]
func (s *Server) postContainersCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if err := checkForJSON(r); err != nil {
		return err
	}

	name := r.Form.Get("name")

	config, hostConfig, err := runconfig.DecodeContainerConfig(r.Body)
	if err != nil {
		return err
	}
	version := versionFromContext(ctx)
	adjustCPUShares := version.LessThan("1.19")

	ccr, err := s.daemon.ContainerCreate(name, config, hostConfig, adjustCPUShares)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusCreated, ccr)
}

// @Title deleteContainers
// @Description Create a new container from an image
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container name"
// @Param   force       query   boolean    false        "Force remove the container"
// @Param   v           query   boolean    false        "Remove the volumes associated to the container"
// @Param   link        query   boolean    false        "Remove the links to other containers"
// @Success 204
// @SubApi /containers
// @Router /containers/:name [delete]
func (s *Server) deleteContainers(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	name := vars["name"]
	config := &daemon.ContainerRmConfig{
		ForceRemove:  boolValue(r, "force"),
		RemoveVolume: boolValue(r, "v"),
		RemoveLink:   boolValue(r, "link"),
	}

	if err := s.daemon.ContainerRm(name, config); err != nil {
		// Force a 404 for the empty string
		if strings.Contains(strings.ToLower(err.Error()), "prefix can't be empty") {
			return fmt.Errorf("no such id: \"\"")
		}
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

// @Title postContainersResize
// @Description Change the tty size of the container
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container name"
// @Param   h           query   integer    false        "High of the tty"
// @Param   w           query   integer    false        "Width of the tty"
// @Success 200
// @SubApi /containers
// @Router /containers/:name/resize [post]
func (s *Server) postContainersResize(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		return err
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		return err
	}

	return s.daemon.ContainerResize(vars["name"], height, width)
}

// @Title postContainersAttach
// @Description Attach to the container's tty
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container name"
// @Param   stdin       query   boolean    false        "Attach stdin to the container"
// @Param   stdout      query   boolean    false        "Attach stdout to the container"
// @Param   stderr      query   boolean    false        "Attach stderr to the container"
// @Param   logs        query   boolean    false        "Attach logs from the container"
// @Param   stream      query   boolean    false        "Stream logs from the container"
// @Success 200
// @SubApi /containers
// @Router /containers/:name/attach [post]
func (s *Server) postContainersAttach(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	containerName := vars["name"]

	if !s.daemon.Exists(containerName) {
		return derr.ErrorCodeNoSuchContainer.WithArgs(containerName)
	}

	inStream, outStream, err := hijackServer(w)
	if err != nil {
		return err
	}
	defer closeStreams(inStream, outStream)

	if _, ok := r.Header["Upgrade"]; ok {
		fmt.Fprintf(outStream, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
	} else {
		fmt.Fprintf(outStream, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	}

	attachWithLogsConfig := &daemon.ContainerAttachWithLogsConfig{
		InStream:  inStream,
		OutStream: outStream,
		UseStdin:  boolValue(r, "stdin"),
		UseStdout: boolValue(r, "stdout"),
		UseStderr: boolValue(r, "stderr"),
		Logs:      boolValue(r, "logs"),
		Stream:    boolValue(r, "stream"),
	}

	if err := s.daemon.ContainerAttachWithLogs(containerName, attachWithLogsConfig); err != nil {
		fmt.Fprintf(outStream, "Error attaching: %s\n", err)
	}

	return nil
}

// @Title wsContainersAttach
// @Description Attach to the container via a websocket
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container name"
// @Param   logs        query   boolean    false        "Attach logs from the container"
// @Param   stream      query   boolean    false        "Stream logs from the container"
// @Success 200
// @SubApi /containers
// @Router /containers/:name/attach [post]
func (s *Server) wsContainersAttach(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	containerName := vars["name"]

	if !s.daemon.Exists(containerName) {
		return derr.ErrorCodeNoSuchContainer.WithArgs(containerName)
	}

	h := websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		wsAttachWithLogsConfig := &daemon.ContainerWsAttachWithLogsConfig{
			InStream:  ws,
			OutStream: ws,
			ErrStream: ws,
			Logs:      boolValue(r, "logs"),
			Stream:    boolValue(r, "stream"),
		}

		if err := s.daemon.ContainerWsAttachWithLogs(containerName, wsAttachWithLogsConfig); err != nil {
			logrus.Errorf("Error attaching websocket: %s", err)
		}
	})
	ws := websocket.Server{Handler: h, Handshake: nil}
	ws.ServeHTTP(w, r)

	return nil
}
