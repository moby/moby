package container // import "github.com/docker/docker/api/server/router/container"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/containerd/log"
	"github.com/docker/docker/api/server/httpstatus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	containerpkg "github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/runconfig"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/websocket"
)

func (s *containerRouter) postCommit(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	config, _, _, err := s.decoder.DecodeConfig(r.Body)
	if err != nil && !errors.Is(err, io.EOF) { // Do not fail if body is empty.
		return err
	}

	ref, err := httputils.RepoTagReference(r.Form.Get("repo"), r.Form.Get("tag"))
	if err != nil {
		return errdefs.InvalidParameter(err)
	}

	imgID, err := s.backend.CreateImageFromContainer(ctx, r.Form.Get("container"), &backend.CreateImageConfig{
		Pause:   httputils.BoolValueOrDefault(r, "pause", true), // TODO(dnephin): remove pause arg, and always pause in backend
		Tag:     ref,
		Author:  r.Form.Get("author"),
		Comment: r.Form.Get("comment"),
		Config:  config,
		Changes: r.Form["changes"],
	})
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

	config := &container.ListOptions{
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

	containers, err := s.backend.Containers(ctx, config)
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

	return s.backend.ContainerStats(ctx, vars["name"], &backend.ContainerStatsConfig{
		Stream:  stream,
		OneShot: oneShot,
		OutStream: func() io.Writer {
			// Assume that when this is called the request is OK.
			w.WriteHeader(http.StatusOK)
			if !stream {
				return w
			}
			wf := ioutils.NewWriteFlusher(w)
			wf.Flush()
			return wf
		},
	})
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
	logsConfig := &container.LogsOptions{
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

	contentType := types.MediaTypeRawStream
	if !tty && versions.GreaterThanOrEqualTo(httputils.VersionFromContext(ctx), "1.42") {
		contentType = types.MediaTypeMultiplexedStream
	}
	w.Header().Set("Content-Type", contentType)

	// if has a tty, we're not muxing streams. if it doesn't, we are. simple.
	// this is the point of no return for writing a response. once we call
	// WriteLogStream, the response has been started and errors will be
	// returned in band by WriteLogStream
	httputils.WriteLogStream(ctx, w, msgs, logsConfig, !tty)
	return nil
}

func (s *containerRouter) getContainersExport(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return s.backend.ContainerExport(ctx, vars["name"], w)
}

func (s *containerRouter) postContainersStart(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	// If contentLength is -1, we can assumed chunked encoding
	// or more technically that the length is unknown
	// https://golang.org/src/pkg/net/http/request.go#L139
	// net/http otherwise seems to swallow any headers related to chunked encoding
	// including r.TransferEncoding
	// allow a nil body for backwards compatibility
	//
	// A non-nil json object is at least 7 characters.
	if r.ContentLength > 7 || r.ContentLength == -1 {
		return errdefs.InvalidParameter(errors.New("starting container with non-empty request body was deprecated since API v1.22 and removed in v1.24"))
	}

	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := s.backend.ContainerStart(ctx, vars["name"], r.Form.Get("checkpoint"), r.Form.Get("checkpoint-dir")); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *containerRouter) postContainersStop(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var (
		options container.StopOptions
		version = httputils.VersionFromContext(ctx)
	)
	if versions.GreaterThanOrEqualTo(version, "1.42") {
		options.Signal = r.Form.Get("signal")
	}
	if tmpSeconds := r.Form.Get("t"); tmpSeconds != "" {
		valSeconds, err := strconv.Atoi(tmpSeconds)
		if err != nil {
			return err
		}
		options.Timeout = &valSeconds
	}

	if err := s.backend.ContainerStop(ctx, vars["name"], options); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *containerRouter) postContainersKill(_ context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	if err := s.backend.ContainerKill(name, r.Form.Get("signal")); err != nil {
		return errors.Wrapf(err, "cannot kill container: %s", name)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *containerRouter) postContainersRestart(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var (
		options container.StopOptions
		version = httputils.VersionFromContext(ctx)
	)
	if versions.GreaterThanOrEqualTo(version, "1.42") {
		options.Signal = r.Form.Get("signal")
	}
	if tmpSeconds := r.Form.Get("t"); tmpSeconds != "" {
		valSeconds, err := strconv.Atoi(tmpSeconds)
		if err != nil {
			return err
		}
		options.Timeout = &valSeconds
	}

	if err := s.backend.ContainerRestart(ctx, vars["name"], options); err != nil {
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
		if v := r.Form.Get("condition"); v != "" {
			switch container.WaitCondition(v) {
			case container.WaitConditionNotRunning:
				waitCondition = containerpkg.WaitConditionNotRunning
			case container.WaitConditionNextExit:
				waitCondition = containerpkg.WaitConditionNextExit
			case container.WaitConditionRemoved:
				waitCondition = containerpkg.WaitConditionRemoved
				legacyRemovalWaitPre134 = versions.LessThan(version, "1.34")
			default:
				return errdefs.InvalidParameter(errors.Errorf("invalid condition: %q", v))
			}
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

	var waitError *container.WaitExitError
	if status.Err() != nil {
		waitError = &container.WaitExitError{Message: status.Err().Error()}
	}

	return json.NewEncoder(w).Encode(&container.WaitResponse{
		StatusCode: int64(status.ExitCode()),
		Error:      waitError,
	})
}

func (s *containerRouter) getContainersChanges(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	changes, err := s.backend.ContainerChanges(ctx, vars["name"])
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

	var updateConfig container.UpdateConfig
	if err := httputils.ReadJSON(r, &updateConfig); err != nil {
		return err
	}
	if versions.LessThan(httputils.VersionFromContext(ctx), "1.40") {
		updateConfig.PidsLimit = nil
	}

	if versions.GreaterThanOrEqualTo(httputils.VersionFromContext(ctx), "1.42") {
		// Ignore KernelMemory removed in API 1.42.
		updateConfig.KernelMemory = 0
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
		if errors.Is(err, io.EOF) {
			return errdefs.InvalidParameter(errors.New("invalid JSON: got EOF while reading request body"))
		}
		return err
	}

	if config == nil {
		return errdefs.InvalidParameter(runconfig.ErrEmptyConfig)
	}
	if hostConfig == nil {
		hostConfig = &container.HostConfig{}
	}
	if networkingConfig == nil {
		networkingConfig = &network.NetworkingConfig{}
	}
	if networkingConfig.EndpointsConfig == nil {
		networkingConfig.EndpointsConfig = make(map[string]*network.EndpointSettings)
	}
	// The NetworkMode "default" is used as a way to express a container should
	// be attached to the OS-dependant default network, in an OS-independent
	// way. Doing this conversion as soon as possible ensures we have less
	// NetworkMode to handle down the path (including in the
	// backward-compatibility layer we have just below).
	//
	// Note that this is not the only place where this conversion has to be
	// done (as there are various other places where containers get created).
	if hostConfig.NetworkMode == "" || hostConfig.NetworkMode.IsDefault() {
		hostConfig.NetworkMode = runconfig.DefaultDaemonNetworkMode()
		if nw, ok := networkingConfig.EndpointsConfig[network.NetworkDefault]; ok {
			networkingConfig.EndpointsConfig[hostConfig.NetworkMode.NetworkName()] = nw
			delete(networkingConfig.EndpointsConfig, network.NetworkDefault)
		}
	}

	version := httputils.VersionFromContext(ctx)

	// When using API 1.24 and under, the client is responsible for removing the container
	if versions.LessThan(version, "1.25") {
		hostConfig.AutoRemove = false
	}

	if versions.LessThan(version, "1.40") {
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
			hostConfig.IpcMode = container.IPCModeShareable
		}
	}

	if versions.LessThan(version, "1.41") {
		// Older clients expect the default to be "host" on cgroup v1 hosts
		if !s.cgroup2 && hostConfig.CgroupnsMode.IsEmpty() {
			hostConfig.CgroupnsMode = container.CgroupnsModeHost
		}
	}

	var platform *ocispec.Platform
	if versions.GreaterThanOrEqualTo(version, "1.41") {
		if v := r.Form.Get("platform"); v != "" {
			p, err := platforms.Parse(v)
			if err != nil {
				return errdefs.InvalidParameter(err)
			}
			platform = &p
		}
	}

	if versions.LessThan(version, "1.42") {
		for _, m := range hostConfig.Mounts {
			// Ignore BindOptions.CreateMountpoint because it was added in API 1.42.
			if bo := m.BindOptions; bo != nil {
				bo.CreateMountpoint = false
			}

			// These combinations are invalid, but weren't validated in API < 1.42.
			// We reset them here, so that validation doesn't produce an error.
			if o := m.VolumeOptions; o != nil && m.Type != mount.TypeVolume {
				m.VolumeOptions = nil
			}
			if o := m.TmpfsOptions; o != nil && m.Type != mount.TypeTmpfs {
				m.TmpfsOptions = nil
			}
			if bo := m.BindOptions; bo != nil {
				// Ignore BindOptions.CreateMountpoint because it was added in API 1.42.
				bo.CreateMountpoint = false
			}
		}

		if runtime.GOOS == "linux" {
			// ConsoleSize is not respected by Linux daemon before API 1.42
			hostConfig.ConsoleSize = [2]uint{0, 0}
		}
	}

	if versions.GreaterThanOrEqualTo(version, "1.42") {
		// Ignore KernelMemory removed in API 1.42.
		hostConfig.KernelMemory = 0
		for _, m := range hostConfig.Mounts {
			if o := m.VolumeOptions; o != nil && m.Type != mount.TypeVolume {
				return errdefs.InvalidParameter(fmt.Errorf("VolumeOptions must not be specified on mount type %q", m.Type))
			}
			if o := m.BindOptions; o != nil && m.Type != mount.TypeBind {
				return errdefs.InvalidParameter(fmt.Errorf("BindOptions must not be specified on mount type %q", m.Type))
			}
			if o := m.TmpfsOptions; o != nil && m.Type != mount.TypeTmpfs {
				return errdefs.InvalidParameter(fmt.Errorf("TmpfsOptions must not be specified on mount type %q", m.Type))
			}
		}
	}

	if versions.LessThan(version, "1.43") {
		// Ignore Annotations because it was added in API v1.43.
		hostConfig.Annotations = nil
	}

	defaultReadOnlyNonRecursive := false
	if versions.LessThan(version, "1.44") {
		if config.Healthcheck != nil {
			// StartInterval was added in API 1.44
			config.Healthcheck.StartInterval = 0
		}

		// Set ReadOnlyNonRecursive to true because it was added in API 1.44
		// Before that all read-only mounts were non-recursive.
		// Keep that behavior for clients on older APIs.
		defaultReadOnlyNonRecursive = true

		for _, m := range hostConfig.Mounts {
			if m.Type == mount.TypeBind {
				if m.BindOptions != nil && m.BindOptions.ReadOnlyForceRecursive {
					// NOTE: that technically this is a breaking change for older
					// API versions, and we should ignore the new field.
					// However, this option may be incorrectly set by a client with
					// the expectation that the failing to apply recursive read-only
					// is enforced, so we decided to produce an error instead,
					// instead of silently ignoring.
					return errdefs.InvalidParameter(errors.New("BindOptions.ReadOnlyForceRecursive needs API v1.44 or newer"))
				}
			}
		}

		// Creating a container connected to several networks is not supported until v1.44.
		if len(networkingConfig.EndpointsConfig) > 1 {
			l := make([]string, 0, len(networkingConfig.EndpointsConfig))
			for k := range networkingConfig.EndpointsConfig {
				l = append(l, k)
			}
			return errdefs.InvalidParameter(errors.Errorf("Container cannot be created with multiple network endpoints: %s", strings.Join(l, ", ")))
		}
	}

	if versions.LessThan(version, "1.45") {
		for _, m := range hostConfig.Mounts {
			if m.VolumeOptions != nil && m.VolumeOptions.Subpath != "" {
				return errdefs.InvalidParameter(errors.New("VolumeOptions.Subpath needs API v1.45 or newer"))
			}
		}
	}

	var warnings []string
	if warn, err := handleMACAddressBC(config, hostConfig, networkingConfig, version); err != nil {
		return err
	} else if warn != "" {
		warnings = append(warnings, warn)
	}

	if hostConfig.PidsLimit != nil && *hostConfig.PidsLimit <= 0 {
		// Don't set a limit if either no limit was specified, or "unlimited" was
		// explicitly set.
		// Both `0` and `-1` are accepted as "unlimited", and historically any
		// negative value was accepted, so treat those as "unlimited" as well.
		hostConfig.PidsLimit = nil
	}

	ccr, err := s.backend.ContainerCreate(ctx, backend.ContainerCreateConfig{
		Name:                        name,
		Config:                      config,
		HostConfig:                  hostConfig,
		NetworkingConfig:            networkingConfig,
		Platform:                    platform,
		DefaultReadOnlyNonRecursive: defaultReadOnlyNonRecursive,
	})
	if err != nil {
		return err
	}
	ccr.Warnings = append(ccr.Warnings, warnings...)
	return httputils.WriteJSON(w, http.StatusCreated, ccr)
}

// handleMACAddressBC takes care of backward-compatibility for the container-wide MAC address by mutating the
// networkingConfig to set the endpoint-specific MACAddress field introduced in API v1.44. It returns a warning message
// or an error if the container-wide field was specified for API >= v1.44.
func handleMACAddressBC(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, version string) (string, error) {
	deprecatedMacAddress := config.MacAddress //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.44.

	// For older versions of the API, migrate the container-wide MAC address to EndpointsConfig.
	if versions.LessThan(version, "1.44") {
		if deprecatedMacAddress == "" {
			// If a MAC address is supplied in EndpointsConfig, discard it because the old API
			// would have ignored it.
			for _, ep := range networkingConfig.EndpointsConfig {
				ep.MacAddress = ""
			}
			return "", nil
		}
		if !hostConfig.NetworkMode.IsBridge() && !hostConfig.NetworkMode.IsUserDefined() {
			return "", runconfig.ErrConflictContainerNetworkAndMac
		}

		// There cannot be more than one entry in EndpointsConfig with API < 1.44.

		// If there's no EndpointsConfig, create a place to store the configured address. It is
		// safe to use NetworkMode as the network name, whether it's a name or id/short-id, as
		// it will be normalised later and there is no other EndpointSettings object that might
		// refer to this network/endpoint.
		if len(networkingConfig.EndpointsConfig) == 0 {
			nwName := hostConfig.NetworkMode.NetworkName()
			networkingConfig.EndpointsConfig[nwName] = &network.EndpointSettings{}
		}
		// There's exactly one network in EndpointsConfig, either from the API or just-created.
		// Migrate the container-wide setting to it.
		// No need to check for a match between NetworkMode and the names/ids in EndpointsConfig,
		// the old version of the API would have applied the address to this network anyway.
		for _, ep := range networkingConfig.EndpointsConfig {
			ep.MacAddress = deprecatedMacAddress
		}
		return "", nil
	}

	// The container-wide MacAddress parameter is deprecated and should now be specified in EndpointsConfig.
	if deprecatedMacAddress == "" {
		return "", nil
	}
	var warning string
	if hostConfig.NetworkMode.IsBridge() || hostConfig.NetworkMode.IsUserDefined() {
		nwName := hostConfig.NetworkMode.NetworkName()
		// If there's no endpoint config, create a place to store the configured address.
		if len(networkingConfig.EndpointsConfig) == 0 {
			networkingConfig.EndpointsConfig[nwName] = &network.EndpointSettings{
				MacAddress: deprecatedMacAddress,
			}
		} else {
			// There is existing endpoint config - if it's not indexed by NetworkMode.Name(), we
			// can't tell which network the container-wide settings was intended for. NetworkMode,
			// the keys in EndpointsConfig and the NetworkID in EndpointsConfig may mix network
			// name/id/short-id. It's not safe to create EndpointsConfig under the NetworkMode
			// name to store the container-wide MAC address, because that may result in two sets
			// of EndpointsConfig for the same network and one set will be discarded later. So,
			// reject the request ...
			ep, ok := networkingConfig.EndpointsConfig[nwName]
			if !ok {
				return "", errdefs.InvalidParameter(errors.New("if a container-wide MAC address is supplied, HostConfig.NetworkMode must match the identity of a network in NetworkSettings.Networks"))
			}
			// ep is the endpoint that needs the container-wide MAC address; migrate the address
			// to it, or bail out if there's a mismatch.
			if ep.MacAddress == "" {
				ep.MacAddress = deprecatedMacAddress
			} else if ep.MacAddress != deprecatedMacAddress {
				return "", errdefs.InvalidParameter(errors.New("the container-wide MAC address must match the endpoint-specific MAC address for the main network, or be left empty"))
			}
		}
	}
	warning = "The container-wide MacAddress field is now deprecated. It should be specified in EndpointsConfig instead."
	config.MacAddress = "" //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.44.

	return warning, nil
}

func (s *containerRouter) deleteContainers(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	config := &backend.ContainerRmConfig{
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

	contentType := types.MediaTypeRawStream
	setupStreams := func(multiplexed bool) (io.ReadCloser, io.Writer, io.Writer, error) {
		conn, _, err := hijacker.Hijack()
		if err != nil {
			return nil, nil, nil, err
		}

		// set raw mode
		conn.Write([]byte{})

		if upgrade {
			if multiplexed && versions.GreaterThanOrEqualTo(httputils.VersionFromContext(ctx), "1.42") {
				contentType = types.MediaTypeMultiplexedStream
			}
			fmt.Fprintf(conn, "HTTP/1.1 101 UPGRADED\r\nContent-Type: "+contentType+"\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
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
		log.G(ctx).WithError(err).Errorf("Handler for %s %s returned error", r.Method, r.URL.Path)
		// Remember to close stream if error happens
		conn, _, errHijack := hijacker.Hijack()
		if errHijack != nil {
			log.G(ctx).WithError(err).Errorf("Handler for %s %s: unable to close stream; error when hijacking connection", r.Method, r.URL.Path)
		} else {
			statusCode := httpstatus.FromError(err)
			statusText := http.StatusText(statusCode)
			fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Type: %s\r\n\r\n%s\r\n", statusCode, statusText, contentType, err.Error())
			httputils.CloseStreams(conn)
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

	setupStreams := func(multiplexed bool) (io.ReadCloser, io.Writer, io.Writer, error) {
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

	useStdin, useStdout, useStderr := true, true, true
	if versions.GreaterThanOrEqualTo(version, "1.42") {
		useStdin = httputils.BoolValue(r, "stdin")
		useStdout = httputils.BoolValue(r, "stdout")
		useStderr = httputils.BoolValue(r, "stderr")
	}

	attachConfig := &backend.ContainerAttachConfig{
		GetStreams: setupStreams,
		UseStdin:   useStdin,
		UseStdout:  useStdout,
		UseStderr:  useStderr,
		Logs:       httputils.BoolValue(r, "logs"),
		Stream:     httputils.BoolValue(r, "stream"),
		DetachKeys: detachKeys,
		MuxStreams: false, // never multiplex, as we rely on websocket to manage distinct streams
	}

	err = s.backend.ContainerAttach(containerName, attachConfig)
	close(done)
	select {
	case <-started:
		if err != nil {
			log.G(ctx).Errorf("Error attaching websocket: %s", err)
		} else {
			log.G(ctx).Debug("websocket connection was closed by client")
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
		return err
	}

	pruneReport, err := s.backend.ContainersPrune(ctx, pruneFilters)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, pruneReport)
}
