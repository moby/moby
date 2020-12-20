package container // import "github.com/docker/docker/api/server/router/container"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"syscall"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	containerpkg "github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/signal"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

func (s *containerRouter) postCommit(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	// TODO: remove pause arg, and always pause in backend
	pause := httputils.BoolValue(r, "pause")
	version := httputils.VersionFromContext(ctx)
	if r.FormValue("pause") == "" && versions.GreaterThanOrEqualTo(version, "1.13") {
		pause = true
	}

	config, _, _, err := s.decoder.DecodeConfig(r.Body)
	if err != nil && err != io.EOF { // Do not fail if body is empty.
		return err
	}

	commitCfg := &backend.CreateImageConfig{
		Pause:   pause,
		Repo:    r.Form.Get("repo"),
		Tag:     r.Form.Get("tag"),
		Author:  r.Form.Get("author"),
		Comment: r.Form.Get("comment"),
		Config:  config,
		Changes: r.Form["changes"],
	}

	imgID, err := s.backend.CreateImageFromContainer(r.Form.Get("container"), commitCfg)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusCreated, &types.IDResponse{ID: imgID})
}

func (s *containerRouter) getContainersJSON(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	filter, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	config := &types.ContainerListOptions{
		All:     httputils.BoolValue(r, "all"),
		Size:    httputils.BoolValue(r, "size"),
		Since:   r.Form.Get("since"),
		Before:  r.Form.Get("before"),
		Filters: filter,
	}

	if tmpLimit := r.Form.Get("limit"); tmpLimit != "" {
		limit, err := strconv.Atoi(tmpLimit)
		if err != nil {
			return err
		}
		config.Limit = limit
	}

	containers, err := s.backend.Containers(config)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, containers)
}

func (s *containerRouter) getContainersStats(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	stream := httputils.BoolValueOrDefault(r, "stream", true)
	if !stream {
		w.Header().Set("Content-Type", "application/json")
	}
	var oneShot bool
	if versions.GreaterThanOrEqualTo(httputils.VersionFromContext(ctx), "1.41") {
		oneShot = httputils.BoolValueOrDefault(r, "one-shot", false)
	}

	config := &backend.ContainerStatsConfig{
		Stream:    stream,
		OneShot:   oneShot,
		OutStream: w,
		Version:   httputils.VersionFromContext(ctx),
	}

	return s.backend.ContainerStats(ctx, vars["name"], config)
}

func (s *containerRouter) getContainersLogs(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	// Args are validated before the stream starts because when it starts we're
	// sending HTTP 200 by writing an empty chunk of data to tell the client that
	// daemon is going to stream. By sending this initial HTTP 200 we can't report
	// any error after the stream starts (i.e. container not found, wrong parameters)
	// with the appropriate status code.
	stdout, stderr := httputils.BoolValue(r, "stdout"), httputils.BoolValue(r, "stderr")
	if !(stdout || stderr) {
		return errdefs.InvalidParameter(errors.New("Bad parameters: you must choose at least one stream"))
	}

	containerName := vars["name"]
	logsConfig := &types.ContainerLogsOptions{
		Follow:     httputils.BoolValue(r, "follow"),
		Timestamps: httputils.BoolValue(r, "timestamps"),
		Since:      r.Form.Get("since"),
		Until:      r.Form.Get("until"),
		Tail:       r.Form.Get("tail"),
		ShowStdout: stdout,
		ShowStderr: stderr,
		Details:    httputils.BoolValue(r, "details"),
	}

	msgs, tty, err := s.backend.ContainerLogs(ctx, containerName, logsConfig)
	if err != nil {
		return err
	}

	// if has a tty, we're not muxing streams. if it doesn't, we are. simple.
	// this is the point of no return for writing a response. once we call
	// WriteLogStream, the response has been started and errors will be
	// returned in band by WriteLogStream
	httputils.WriteLogStream(ctx, w, msgs, logsConfig, !tty)
	return nil
}

func (s *containerRouter) getContainersExport(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return s.backend.ContainerExport(vars["name"], w)
}

type bodyOnStartError struct{}

func (bodyOnStartError) Error() string {
	return "starting container with non-empty request body was deprecated since API v1.22 and removed in v1.24"
}

func (bodyOnStartError) InvalidParameter() {}

func (s *containerRouter) postContainersStart(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	// If contentLength is -1, we can assumed chunked encoding
	// or more technically that the length is unknown
	// https://golang.org/src/pkg/net/http/request.go#L139
	// net/http otherwise seems to swallow any headers related to chunked encoding
	// including r.TransferEncoding
	// allow a nil body for backwards compatibility

	version := httputils.VersionFromContext(ctx)
	var hostConfig *container.HostConfig
	// A non-nil json object is at least 7 characters.
	if r.ContentLength > 7 || r.ContentLength == -1 {
		if versions.GreaterThanOrEqualTo(version, "1.24") {
			return bodyOnStartError{}
		}

		if err := httputils.CheckForJSON(r); err != nil {
			return err
		}

		c, err := s.decoder.DecodeHostConfig(r.Body)
		if err != nil {
			return err
		}
		hostConfig = c
	}

	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	checkpoint := r.Form.Get("checkpoint")
	checkpointDir := r.Form.Get("checkpoint-dir")
	if err := s.backend.ContainerStart(vars["name"], hostConfig, checkpoint, checkpointDir); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *containerRouter) postContainersStop(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var seconds *int
	if tmpSeconds := r.Form.Get("t"); tmpSeconds != "" {
		valSeconds, err := strconv.Atoi(tmpSeconds)
		if err != nil {
			return err
		}
		seconds = &valSeconds
	}

	if err := s.backend.ContainerStop(vars["name"], seconds); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)

	return nil
}

func (s *containerRouter) postContainersKill(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var sig syscall.Signal
	name := vars["name"]

	// If we have a signal, look at it. Otherwise, do nothing
	if sigStr := r.Form.Get("signal"); sigStr != "" {
		var err error
		if sig, err = signal.ParseSignal(sigStr); err != nil {
			return errdefs.InvalidParameter(err)
		}
	}

	if err := s.backend.ContainerKill(name, uint64(sig)); err != nil {
		var isStopped bool
		if errdefs.IsConflict(err) {
			isStopped = true
		}

		// Return error that's not caused because the container is stopped.
		// Return error if the container is not running and the api is >= 1.20
		// to keep backwards compatibility.
		version := httputils.VersionFromContext(ctx)
		if versions.GreaterThanOrEqualTo(version, "1.20") || !isStopped {
			return errors.Wrapf(err, "Cannot kill container: %s", name)
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *containerRouter) postContainersRestart(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var seconds *int
	if tmpSeconds := r.Form.Get("t"); tmpSeconds != "" {
		valSeconds, err := strconv.Atoi(tmpSeconds)
		if err != nil {
			return err
		}
		seconds = &valSeconds
	}

	if err := s.backend.ContainerRestart(vars["name"], seconds); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

func (s *containerRouter) postContainersPause(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := s.backend.ContainerPause(vars["name"]); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

func (s *containerRouter) postContainersUnpause(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := s.backend.ContainerUnpause(vars["name"]); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

func (s *containerRouter) postContainersWait(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	// Behavior changed in version 1.30 to handle wait condition and to
	// return headers immediately.
	version := httputils.VersionFromContext(ctx)
	legacyBehaviorPre130 := versions.LessThan(version, "1.30")
	legacyRemovalWaitPre134 := false

	// The wait condition defaults to "not-running".
	waitCondition := containerpkg.WaitConditionNotRunning
	if !legacyBehaviorPre130 {
		if err := httputils.ParseForm(r); err != nil {
			return err
		}
		switch container.WaitCondition(r.Form.Get("condition")) {
		case container.WaitConditionNextExit:
			waitCondition = containerpkg.WaitConditionNextExit
		case container.WaitConditionRemoved:
			waitCondition = containerpkg.WaitConditionRemoved
			legacyRemovalWaitPre134 = versions.LessThan(version, "1.34")
		}
	}

	waitC, err := s.backend.ContainerWait(ctx, vars["name"], waitCondition)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")

	if !legacyBehaviorPre130 {
		// Write response header immediately.
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// Block on the result of the wait operation.
	status := <-waitC

	// With API < 1.34, wait on WaitConditionRemoved did not return
	// in case container removal failed. The only way to report an
	// error back to the client is to not write anything (i.e. send
	// an empty response which will be treated as an error).
	if legacyRemovalWaitPre134 && status.Err() != nil {
		return nil
	}

	var waitError *container.ContainerWaitOKBodyError
	if status.Err() != nil {
		waitError = &container.ContainerWaitOKBodyError{Message: status.Err().Error()}
	}

	return json.NewEncoder(w).Encode(&container.ContainerWaitOKBody{
		StatusCode: int64(status.ExitCode()),
		Error:      waitError,
	})
}

func (s *containerRouter) getContainersChanges(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	changes, err := s.backend.ContainerChanges(vars["name"])
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, changes)
}

func (s *containerRouter) getContainersTop(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	procList, err := s.backend.ContainerTop(vars["name"], r.Form.Get("ps_args"))
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, procList)
}

func (s *containerRouter) postContainerRename(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	newName := r.Form.Get("name")
	if err := s.backend.ContainerRename(name, newName); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *containerRouter) postContainerUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	var updateConfig container.UpdateConfig

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&updateConfig); err != nil {
		return err
	}
	if versions.LessThan(httputils.VersionFromContext(ctx), "1.40") {
		updateConfig.PidsLimit = nil
	}
	if updateConfig.PidsLimit != nil && *updateConfig.PidsLimit <= 0 {
		// Both `0` and `-1` are accepted to set "unlimited" when updating.
		// Historically, any negative value was accepted, so treat them as
		// "unlimited" as well.
		var unlimited int64
		updateConfig.PidsLimit = &unlimited
	}

	hostConfig := &container.HostConfig{
		Resources:     updateConfig.Resources,
		RestartPolicy: updateConfig.RestartPolicy,
	}

	name := vars["name"]
	resp, err := s.backend.ContainerUpdate(name, hostConfig)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, resp)
}

func (s *containerRouter) postContainersCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	name := r.Form.Get("name")

	config, hostConfig, networkingConfig, err := s.decoder.DecodeConfig(r.Body)
	if err != nil {
		return err
	}
	version := httputils.VersionFromContext(ctx)
	adjustCPUShares := versions.LessThan(version, "1.19")

	// When using API 1.24 and under, the client is responsible for removing the container
	if hostConfig != nil && versions.LessThan(version, "1.25") {
		hostConfig.AutoRemove = false
	}

	if hostConfig != nil && versions.LessThan(version, "1.40") {
		// Ignore BindOptions.NonRecursive because it was added in API 1.40.
		for _, m := range hostConfig.Mounts {
			if bo := m.BindOptions; bo != nil {
				bo.NonRecursive = false
			}
		}
		// Ignore KernelMemoryTCP because it was added in API 1.40.
		hostConfig.KernelMemoryTCP = 0

		// Older clients (API < 1.40) expects the default to be shareable, make them happy
		if hostConfig.IpcMode.IsEmpty() {
			hostConfig.IpcMode = container.IpcMode("shareable")
		}
	}
	if hostConfig != nil && versions.LessThan(version, "1.41") && !s.cgroup2 {
		// Older clients expect the default to be "host" on cgroup v1 hosts
		if hostConfig.CgroupnsMode.IsEmpty() {
			hostConfig.CgroupnsMode = container.CgroupnsMode("host")
		}
	}

	var platform *specs.Platform
	if versions.GreaterThanOrEqualTo(version, "1.41") {
		if v := r.Form.Get("platform"); v != "" {
			p, err := platforms.Parse(v)
			if err != nil {
				return errdefs.InvalidParameter(err)
			}
			platform = &p
		}
	}

	if hostConfig != nil && hostConfig.PidsLimit != nil && *hostConfig.PidsLimit <= 0 {
		// Don't set a limit if either no limit was specified, or "unlimited" was
		// explicitly set.
		// Both `0` and `-1` are accepted as "unlimited", and historically any
		// negative value was accepted, so treat those as "unlimited" as well.
		hostConfig.PidsLimit = nil
	}

	ccr, err := s.backend.ContainerCreate(types.ContainerCreateConfig{
		Name:             name,
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		AdjustCPUShares:  adjustCPUShares,
		Platform:         platform,
	})
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusCreated, ccr)
}

func (s *containerRouter) deleteContainers(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	config := &types.ContainerRmConfig{
		ForceRemove:  httputils.BoolValue(r, "force"),
		RemoveVolume: httputils.BoolValue(r, "v"),
		RemoveLink:   httputils.BoolValue(r, "link"),
	}

	if err := s.backend.ContainerRm(name, config); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}

func (s *containerRouter) postContainersResize(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		return errdefs.InvalidParameter(err)
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		return errdefs.InvalidParameter(err)
	}

	return s.backend.ContainerResize(vars["name"], height, width)
}

func (s *containerRouter) postContainersAttach(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	err := httputils.ParseForm(r)
	if err != nil {
		return err
	}
	containerName := vars["name"]

	_, upgrade := r.Header["Upgrade"]
	detachKeys := r.FormValue("detachKeys")

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return errdefs.InvalidParameter(errors.Errorf("error attaching to container %s, hijack connection missing", containerName))
	}

	setupStreams := func() (io.ReadCloser, io.Writer, io.Writer, error) {
		conn, _, err := hijacker.Hijack()
		if err != nil {
			return nil, nil, nil, err
		}

		// set raw mode
		conn.Write([]byte{})

		if upgrade {
			fmt.Fprintf(conn, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
		} else {
			fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
		}

		closer := func() error {
			httputils.CloseStreams(conn)
			return nil
		}
		return ioutils.NewReadCloserWrapper(conn, closer), conn, conn, nil
	}

	attachConfig := &backend.ContainerAttachConfig{
		GetStreams: setupStreams,
		UseStdin:   httputils.BoolValue(r, "stdin"),
		UseStdout:  httputils.BoolValue(r, "stdout"),
		UseStderr:  httputils.BoolValue(r, "stderr"),
		Logs:       httputils.BoolValue(r, "logs"),
		Stream:     httputils.BoolValue(r, "stream"),
		DetachKeys: detachKeys,
		MuxStreams: true,
	}

	if err = s.backend.ContainerAttach(containerName, attachConfig); err != nil {
		logrus.Errorf("Handler for %s %s returned error: %v", r.Method, r.URL.Path, err)
		// Remember to close stream if error happens
		conn, _, errHijack := hijacker.Hijack()
		if errHijack == nil {
			statusCode := errdefs.GetHTTPErrorStatusCode(err)
			statusText := http.StatusText(statusCode)
			fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n%s\r\n", statusCode, statusText, err.Error())
			httputils.CloseStreams(conn)
		} else {
			logrus.Errorf("Error Hijacking: %v", err)
		}
	}
	return nil
}

func (s *containerRouter) wsContainersAttach(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	containerName := vars["name"]

	var err error
	detachKeys := r.FormValue("detachKeys")

	done := make(chan struct{})
	started := make(chan struct{})

	version := httputils.VersionFromContext(ctx)

	setupStreams := func() (io.ReadCloser, io.Writer, io.Writer, error) {
		wsChan := make(chan *websocket.Conn)
		h := func(conn *websocket.Conn) {
			wsChan <- conn
			<-done
		}

		srv := websocket.Server{Handler: h, Handshake: nil}
		go func() {
			close(started)
			srv.ServeHTTP(w, r)
		}()

		conn := <-wsChan
		// In case version 1.28 and above, a binary frame will be sent.
		// See 28176 for details.
		if versions.GreaterThanOrEqualTo(version, "1.28") {
			conn.PayloadType = websocket.BinaryFrame
		}
		return conn, conn, conn, nil
	}

	attachConfig := &backend.ContainerAttachConfig{
		GetStreams: setupStreams,
		Logs:       httputils.BoolValue(r, "logs"),
		Stream:     httputils.BoolValue(r, "stream"),
		DetachKeys: detachKeys,
		UseStdin:   true,
		UseStdout:  true,
		UseStderr:  true,
		MuxStreams: false, // TODO: this should be true since it's a single stream for both stdout and stderr
	}

	err = s.backend.ContainerAttach(containerName, attachConfig)
	close(done)
	select {
	case <-started:
		if err != nil {
			logrus.Errorf("Error attaching websocket: %s", err)
		} else {
			logrus.Debug("websocket connection was closed by client")
		}
		return nil
	default:
	}
	return err
}

func (s *containerRouter) postContainersPrune(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	pruneFilters, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return errdefs.InvalidParameter(err)
	}

	pruneReport, err := s.backend.ContainersPrune(ctx, pruneFilters)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, pruneReport)
}
