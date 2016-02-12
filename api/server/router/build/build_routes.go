package build

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/utils"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/go-units"
	"golang.org/x/net/context"
)

func newImageBuildOptions(ctx context.Context, r *http.Request) (*types.ImageBuildOptions, error) {
	version := httputils.VersionFromContext(ctx)
	options := &types.ImageBuildOptions{}
	if httputils.BoolValue(r, "forcerm") && version.GreaterThanOrEqualTo("1.12") {
		options.Remove = true
	} else if r.FormValue("rm") == "" && version.GreaterThanOrEqualTo("1.12") {
		options.Remove = true
	} else {
		options.Remove = httputils.BoolValue(r, "rm")
	}
	if httputils.BoolValue(r, "pull") && version.GreaterThanOrEqualTo("1.16") {
		options.PullParent = true
	}

	options.Dockerfile = r.FormValue("dockerfile")
	options.SuppressOutput = httputils.BoolValue(r, "q")
	options.NoCache = httputils.BoolValue(r, "nocache")
	options.ForceRemove = httputils.BoolValue(r, "forcerm")
	options.MemorySwap = httputils.Int64ValueOrZero(r, "memswap")
	options.Memory = httputils.Int64ValueOrZero(r, "memory")
	options.CPUShares = httputils.Int64ValueOrZero(r, "cpushares")
	options.CPUPeriod = httputils.Int64ValueOrZero(r, "cpuperiod")
	options.CPUQuota = httputils.Int64ValueOrZero(r, "cpuquota")
	options.CPUSetCPUs = r.FormValue("cpusetcpus")
	options.CPUSetMems = r.FormValue("cpusetmems")
	options.CgroupParent = r.FormValue("cgroupparent")
	options.Tags = r.Form["t"]

	if r.Form.Get("shmsize") != "" {
		shmSize, err := strconv.ParseInt(r.Form.Get("shmsize"), 10, 64)
		if err != nil {
			return nil, err
		}
		options.ShmSize = shmSize
	}

	if i := container.Isolation(r.FormValue("isolation")); i != "" {
		if !container.Isolation.IsValid(i) {
			return nil, fmt.Errorf("Unsupported isolation: %q", i)
		}
		options.Isolation = i
	}

	var buildUlimits = []*units.Ulimit{}
	ulimitsJSON := r.FormValue("ulimits")
	if ulimitsJSON != "" {
		if err := json.NewDecoder(strings.NewReader(ulimitsJSON)).Decode(&buildUlimits); err != nil {
			return nil, err
		}
		options.Ulimits = buildUlimits
	}

	var buildArgs = map[string]string{}
	buildArgsJSON := r.FormValue("buildargs")
	if buildArgsJSON != "" {
		if err := json.NewDecoder(strings.NewReader(buildArgsJSON)).Decode(&buildArgs); err != nil {
			return nil, err
		}
		options.BuildArgs = buildArgs
	}
	return options, nil
}

func (br *buildRouter) postBuild(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var (
		authConfigs        = map[string]types.AuthConfig{}
		authConfigsEncoded = r.Header.Get("X-Registry-Config")
		notVerboseBuffer   = bytes.NewBuffer(nil)
	)

	if authConfigsEncoded != "" {
		authConfigsJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authConfigsEncoded))
		if err := json.NewDecoder(authConfigsJSON).Decode(&authConfigs); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting
			// to be empty.
		}
	}

	w.Header().Set("Content-Type", "application/json")

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	sf := streamformatter.NewJSONStreamFormatter()
	errf := func(err error) error {
		if httputils.BoolValue(r, "q") && notVerboseBuffer.Len() > 0 {
			output.Write(notVerboseBuffer.Bytes())
		}
		// Do not write the error in the http output if it's still empty.
		// This prevents from writing a 200(OK) when there is an internal error.
		if !output.Flushed() {
			return err
		}
		_, err = w.Write(sf.FormatError(errors.New(utils.GetErrorMessage(err))))
		if err != nil {
			logrus.Warnf("could not write error response: %v", err)
		}
		return nil
	}

	buildOptions, err := newImageBuildOptions(ctx, r)
	if err != nil {
		return errf(err)
	}

	remoteURL := r.FormValue("remote")

	// Currently, only used if context is from a remote url.
	// Look at code in DetectContextFromRemoteURL for more information.
	createProgressReader := func(in io.ReadCloser) io.ReadCloser {
		progressOutput := sf.NewProgressOutput(output, true)
		if buildOptions.SuppressOutput {
			progressOutput = sf.NewProgressOutput(notVerboseBuffer, true)
		}
		return progress.NewProgressReader(in, progressOutput, r.ContentLength, "Downloading context", remoteURL)
	}

	var (
		context        builder.ModifiableContext
		dockerfileName string
		out            io.Writer
	)
	context, dockerfileName, err = builder.DetectContextFromRemoteURL(r.Body, remoteURL, createProgressReader)
	if err != nil {
		return errf(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			logrus.Debugf("[BUILDER] failed to remove temporary context: %v", err)
		}
	}()
	if len(dockerfileName) > 0 {
		buildOptions.Dockerfile = dockerfileName
	}

	buildOptions.AuthConfigs = authConfigs

	out = output
	if buildOptions.SuppressOutput {
		out = notVerboseBuffer
	}
	stdout := &streamformatter.StdoutFormatter{Writer: out, StreamFormatter: sf}
	stderr := &streamformatter.StderrFormatter{Writer: out, StreamFormatter: sf}

	closeNotifier := make(<-chan bool)
	if notifier, ok := w.(http.CloseNotifier); ok {
		closeNotifier = notifier.CloseNotify()
	}

	imgID, err := br.backend.Build(buildOptions,
		builder.DockerIgnoreContext{ModifiableContext: context},
		stdout, stderr, out,
		closeNotifier)
	if err != nil {
		return errf(err)
	}

	// Everything worked so if -q was provided the output from the daemon
	// should be just the image ID and we'll print that to stdout.
	if buildOptions.SuppressOutput {
		stdout := &streamformatter.StdoutFormatter{Writer: output, StreamFormatter: sf}
		fmt.Fprintf(stdout, "%s\n", string(imgID))
	}

	return nil
}
