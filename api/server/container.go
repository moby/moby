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

func (s *Server) getContainersStats(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
	}

	return s.daemon.ContainerStats(vars["name"], config)
}

func (s *Server) getContainersLogs(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	// Validate args here, because we can't return not StatusOK after job.Run() call
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
		fmt.Fprintf(w, "Error running logs job: %s\n", err)
	}

	return nil
}

func (s *Server) getContainersExport(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	return s.daemon.ContainerExport(vars["name"], w)
}

func (s *Server) postContainersStart(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
		if err.Error() == "Container already started" {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) postContainersStop(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	seconds, _ := strconv.Atoi(r.Form.Get("t"))

	if err := s.daemon.ContainerStop(vars["name"], seconds); err != nil {
		if err.Error() == "Container already stopped" {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
		return err
	}
	w.WriteHeader(http.StatusNoContent)

	return nil
}

func (s *Server) postContainersKill(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}

	var sig uint64
	name := vars["name"]

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

	if err := s.daemon.ContainerKill(name, sig); err != nil {
		_, isStopped := err.(daemon.ErrContainerNotRunning)
		// Return error that's not caused because the container is stopped.
		// Return error if the container is not running and the api is >= 1.20
		// to keep backwards compatibility.
		if version.GreaterThanOrEqualTo("1.20") || !isStopped {
			return fmt.Errorf("Cannot kill container %s: %v", name, err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) postContainersRestart(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) postContainersPause(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) postContainersUnpause(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) postContainersWait(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) getContainersChanges(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	changes, err := s.daemon.ContainerChanges(vars["name"])
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, changes)
}

func (s *Server) getContainersTop(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) postContainerRename(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) deleteContainers(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) postContainersResize(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func (s *Server) postContainersAttach(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	cont, err := s.daemon.Get(vars["name"])
	if err != nil {
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

	if err := s.daemon.ContainerAttachWithLogs(cont, attachWithLogsConfig); err != nil {
		fmt.Fprintf(outStream, "Error attaching: %s\n", err)
	}

	return nil
}

func (s *Server) wsContainersAttach(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	cont, err := s.daemon.Get(vars["name"])
	if err != nil {
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

		if err := s.daemon.ContainerWsAttachWithLogs(cont, wsAttachWithLogsConfig); err != nil {
			logrus.Errorf("Error attaching websocket: %s", err)
		}
	})
	ws := websocket.Server{Handler: h, Handshake: nil}
	ws.ServeHTTP(w, r)

	return nil
}
