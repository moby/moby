package server

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/websocket"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/middleware"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/runconfig"
)

func (s *Server) getContainersByName(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	if version.LessThan("1.20") && runtime.GOOS != "windows" {
		return getContainersByNameDownlevel(w, s, vars["name"])
	}

	containerJSON, err := s.daemon.ContainerInspect(vars["name"])
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, containerJSON)
}

func (s *Server) getContainersJSON(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) getContainersStats(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	stream := boolValueOrDefault(r.Request, "stream", true)
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
	}

	return s.daemon.ContainerStats(r.Container, config)
}

func (s *Server) getContainersLogs(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	// Validate args here, because we can't return not StatusOK after job.Run() call
	stdout, stderr := boolValue(r.Request, "stdout"), boolValue(r.Request, "stderr")
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

	outStream := ioutils.NewWriteFlusher(w)
	// write an empty chunk of data (this is to ensure that the
	// HTTP Response is sent immediately, even if the container has
	// not yet produced any data)
	outStream.Write(nil)

	logsConfig := &daemon.ContainerLogsConfig{
		Follow:     boolValue(r.Request, "follow"),
		Timestamps: boolValue(r.Request, "timestamps"),
		Since:      since,
		Tail:       r.Request.Form.Get("tail"),
		UseStdout:  stdout,
		UseStderr:  stderr,
		OutStream:  outStream,
		Stop:       closeNotifier,
	}

	if err := s.daemon.ContainerLogs(r.Container, logsConfig); err != nil {
		fmt.Fprintf(w, "Error running logs job: %s\n", err)
	}

	return nil
}

func (s *Server) getContainersExport(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	data, err := r.Container.Export()
	if err != nil {
		return fmt.Errorf("%s: %s", r.Container.ID, err)
	}
	defer data.Close()

	// Stream the entire contents of the container (basically a volatile snapshot)
	if _, err := io.Copy(w, data); err != nil {
		return fmt.Errorf("%s: %s", r.Container.ID, err)
	}
	return nil
}

func (s *Server) postContainersStart(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	// If contentLength is -1, we can assumed chunked encoding
	// or more technically that the length is unknown
	// https://golang.org/src/pkg/net/http/request.go#L139
	// net/http otherwise seems to swallow any headers related to chunked encoding
	// including r.TransferEncoding
	// allow a nil body for backwards compatibility
	var hostConfig *runconfig.HostConfig
	req := r.Request
	if req.Body != nil && (req.ContentLength > 0 || req.ContentLength == -1) {
		if err := checkForJSON(req); err != nil {
			return err
		}

		c, err := runconfig.DecodeHostConfig(req.Body)
		if err != nil {
			return err
		}

		hostConfig = c
	}

	if err := s.daemon.ContainerStart(r.Container, hostConfig); err != nil {
		if err.Error() == "Container already started" {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) postContainersStop(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	seconds, _ := strconv.Atoi(r.Form.Get("t"))

	if err := s.daemon.ContainerStop(r.Container, seconds); err != nil {
		if err.Error() == "Container already stopped" {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
		return err
	}
	w.WriteHeader(http.StatusNoContent)

	return nil
}

func (s *Server) postContainersKill(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	var sig uint64

	// If we have a signal, look at it. Otherwise, do nothing
	if sigStr := r.Form.Get("signal"); sigStr != "" {
		// Check if we passed the signal as a number:
		// The largest legal signal is 31, so let's parse on 5 bits
		sigN, err := strconv.ParseUint(sigStr, 10, 5)
		if err != nil {
			// The signal is not a number, treat it as a string (either like
			// "KILL" or like "SIGKILL")
			syscallSig, ok := signal.SignalMap[strings.TrimPrefix(sigStr, "SIG")]
			if !ok {
				return fmt.Errorf("Invalid signal: %s", sigStr)
			}
			sig = uint64(syscallSig)
		} else {
			sig = sigN
		}

		if sig == 0 {
			return fmt.Errorf("Invalid signal: %s", sigStr)
		}
	}

	if err := s.daemon.ContainerKill(r.Container, sig); err != nil {
		_, isStopped := err.(daemon.ErrContainerNotRunning)
		// Return error that's not caused because the container is stopped.
		// Return error if the container is not running and the api is >= 1.20
		// to keep backwards compatibility.
		if r.Version.GreaterThanOrEqualTo("1.20") || !isStopped {
			return fmt.Errorf("Cannot kill container %s: %v", r.Container.ID, err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// postContainerRestart stops and starts a container. It attempts to
// gracefully stop the container within the given timeout, forcefully
// stopping it if the timeout is exceeded. If given a negative
// timeout, ContainerRestart will wait forever until a graceful
// stop. Returns an error if the container cannot be found, or if
// there is an underlying error at any stage of the restart.
func (s *Server) postContainersRestart(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	seconds, _ := strconv.Atoi(r.Form.Get("t"))

	if err := r.Container.Restart(seconds); err != nil {
		return fmt.Errorf("Cannot restart container %s: %s\n", r.Container.ID, err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) postContainersPause(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := r.Container.Pause(); err != nil {
		return fmt.Errorf("Cannot pause container %s: %s", r.Container.ID, err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) postContainersUnpause(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := r.Container.Unpause(); err != nil {
		return fmt.Errorf("Cannot unpause container %s: %s", r.Container.ID, err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// postContainersWait stops processing until the given container is
// stopped. If the container is not found, an error is returned. On a
// successful stop, the exit code of the container is returned. On a
// timeout, an error is returned. If you want to wait forever, supply
// a negative duration for the timeout.
func (s *Server) postContainersWait(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	status, err := r.Container.WaitStop(-1 * time.Second)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, &types.ContainerWaitResponse{
		StatusCode: status,
	})
}

func (s *Server) getContainersChanges(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	changes, err := r.Container.Changes()
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, changes)
}

func (s *Server) getContainersTop(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	procList, err := s.daemon.ContainerTop(r.Container, r.Form.Get("ps_args"))
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, procList)
}

func (s *Server) postContainerRename(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	newName := r.Form.Get("name")
	if err := s.daemon.ContainerRename(r.Container, newName); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) postContainersCreate(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if err := checkForJSON(r); err != nil {
		return err
	}
	var (
		warnings []string
		name     = r.Form.Get("name")
	)

	config, hostConfig, err := runconfig.DecodeContainerConfig(r.Body)
	if err != nil {
		return err
	}
	adjustCPUShares := version.LessThan("1.19")

	container, warnings, err := s.daemon.ContainerCreate(name, config, hostConfig, adjustCPUShares)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusCreated, &types.ContainerCreateResponse{
		ID:       container.ID,
		Warnings: warnings,
	})
}

func (s *Server) deleteContainers(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	config := &daemon.ContainerRmConfig{
		ForceRemove:  boolValue(r, "force"),
		RemoveVolume: boolValue(r, "v"),
		RemoveLink:   boolValue(r, "link"),
	}

	if config.RemoveLink {
		name, err := r.GetVar("name")
		if err != nil {
			return err
		}
		name, err = daemon.GetFullContainerName(name)
		if err != nil {
			return err
		}
		config.FullName = name
	}

	if err := s.daemon.ContainerRm(r.Container, config); err != nil {
		// Force a 404 for the empty string
		if strings.Contains(strings.ToLower(err.Error()), "prefix can't be empty") {
			return fmt.Errorf("no such id: \"\"")
		}
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

// postContainerResize changes the size of the TTY of the process running
// in the container with the given name to the given height and width.
func (s *Server) postContainersResize(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
	}

	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		return err
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		return err
	}

	return r.Container.Resize(height, width)
}

func (s *Server) postContainersAttach(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
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

	if err := s.daemon.ContainerAttachWithLogs(r.Container, attachWithLogsConfig); err != nil {
		fmt.Fprintf(outStream, "Error attaching: %s\n", err)
	}

	return nil
}

func (s *Server) wsContainersAttach(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := parseForm(r); err != nil {
		return err
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

		if err := s.daemon.ContainerWsAttachWithLogs(r.Container, wsAttachWithLogsConfig); err != nil {
			logrus.Errorf("Error attaching websocket: %s", err)
		}
	})
	ws := websocket.Server{Handler: h, Handshake: nil}
	ws.ServeHTTP(w, r.Request)

	return nil
}
