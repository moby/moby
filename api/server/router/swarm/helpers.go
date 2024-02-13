package swarm // import "github.com/docker/docker/api/server/router/swarm"

import (
	"context"
	"fmt"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	basictypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/versions"
)

// swarmLogs takes an http response, request, and selector, and writes the logs
// specified by the selector to the response
func (sr *swarmRouter) swarmLogs(ctx context.Context, w http.ResponseWriter, r *http.Request, selector *backend.LogSelector) error {
	// Args are validated before the stream starts because when it starts we're
	// sending HTTP 200 by writing an empty chunk of data to tell the client that
	// daemon is going to stream. By sending this initial HTTP 200 we can't report
	// any error after the stream starts (i.e. container not found, wrong parameters)
	// with the appropriate status code.
	stdout, stderr := httputils.BoolValue(r, "stdout"), httputils.BoolValue(r, "stderr")
	if !(stdout || stderr) {
		return fmt.Errorf("Bad parameters: you must choose at least one stream")
	}

	// there is probably a neater way to manufacture the LogsOptions
	// struct, probably in the caller, to eliminate the dependency on net/http
	logsConfig := &container.LogsOptions{
		Follow:     httputils.BoolValue(r, "follow"),
		Timestamps: httputils.BoolValue(r, "timestamps"),
		Since:      r.Form.Get("since"),
		Tail:       r.Form.Get("tail"),
		ShowStdout: stdout,
		ShowStderr: stderr,
		Details:    httputils.BoolValue(r, "details"),
	}

	tty := false
	// checking for whether logs are TTY involves iterating over every service
	// and task. idk if there is a better way
	for _, service := range selector.Services {
		s, err := sr.backend.GetService(service, false)
		if err != nil {
			// maybe should return some context with this error?
			return err
		}
		tty = (s.Spec.TaskTemplate.ContainerSpec != nil && s.Spec.TaskTemplate.ContainerSpec.TTY) || tty
	}
	for _, task := range selector.Tasks {
		t, err := sr.backend.GetTask(task)
		if err != nil {
			// as above
			return err
		}
		tty = t.Spec.ContainerSpec.TTY || tty
	}

	msgs, err := sr.backend.ServiceLogs(ctx, selector, logsConfig)
	if err != nil {
		return err
	}

	contentType := basictypes.MediaTypeRawStream
	if !tty && versions.GreaterThanOrEqualTo(httputils.VersionFromContext(ctx), "1.42") {
		contentType = basictypes.MediaTypeMultiplexedStream
	}
	w.Header().Set("Content-Type", contentType)
	httputils.WriteLogStream(ctx, w, msgs, logsConfig, !tty)
	return nil
}

// adjustForAPIVersion takes a version and service spec and removes fields to
// make the spec compatible with the specified version.
func adjustForAPIVersion(cliVersion string, service *swarm.ServiceSpec) {
	if cliVersion == "" {
		return
	}
	if versions.LessThan(cliVersion, "1.40") {
		if service.TaskTemplate.ContainerSpec != nil {
			// Sysctls for docker swarm services weren't supported before
			// API version 1.40
			service.TaskTemplate.ContainerSpec.Sysctls = nil

			if service.TaskTemplate.ContainerSpec.Privileges != nil && service.TaskTemplate.ContainerSpec.Privileges.CredentialSpec != nil {
				// Support for setting credential-spec through configs was added in API 1.40
				service.TaskTemplate.ContainerSpec.Privileges.CredentialSpec.Config = ""
			}
			for _, config := range service.TaskTemplate.ContainerSpec.Configs {
				// support for the Runtime target was added in API 1.40
				config.Runtime = nil
			}
		}

		if service.TaskTemplate.Placement != nil {
			// MaxReplicas for docker swarm services weren't supported before
			// API version 1.40
			service.TaskTemplate.Placement.MaxReplicas = 0
		}
	}
	if versions.LessThan(cliVersion, "1.41") {
		if service.TaskTemplate.ContainerSpec != nil {
			// Capabilities and Ulimits for docker swarm services weren't
			// supported before API version 1.41
			service.TaskTemplate.ContainerSpec.CapabilityAdd = nil
			service.TaskTemplate.ContainerSpec.CapabilityDrop = nil
			service.TaskTemplate.ContainerSpec.Ulimits = nil
		}
		if service.TaskTemplate.Resources != nil && service.TaskTemplate.Resources.Limits != nil {
			// Limits.Pids  not supported before API version 1.41
			service.TaskTemplate.Resources.Limits.Pids = 0
		}

		// jobs were only introduced in API version 1.41. Nil out both Job
		// modes; if the service is one of these modes and subsequently has no
		// mode, then something down the pipe will thrown an error.
		service.Mode.ReplicatedJob = nil
		service.Mode.GlobalJob = nil
	}

	if versions.LessThan(cliVersion, "1.44") {
		// seccomp, apparmor, and no_new_privs were added in 1.44.
		if service.TaskTemplate.ContainerSpec != nil && service.TaskTemplate.ContainerSpec.Privileges != nil {
			service.TaskTemplate.ContainerSpec.Privileges.Seccomp = nil
			service.TaskTemplate.ContainerSpec.Privileges.AppArmor = nil
			service.TaskTemplate.ContainerSpec.Privileges.NoNewPrivileges = false
		}
	}
}
