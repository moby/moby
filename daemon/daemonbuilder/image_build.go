package daemonbuilder

import (
	"errors"
	"io"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/utils"
)

// BuildImage uses the docker daemon to build and tag images.
// It detects the dockerfile supplied based on the remoteURL in the configuration.
// When that URL is not supplied, it assumes the body is a tar file and it's extracted on the host.
func BuildImage(daemon *daemon.Daemon, buildConfig *dockerfile.Config, authConfigs map[string]cliconfig.AuthConfig, bodyReader io.ReadCloser, respWriter io.Writer, size int64) error {
	respFlusher := ioutils.NewWriteFlusher(respWriter)
	defer respFlusher.Close()

	context, dockerfileName, err := detectContextFromRemoteURL(bodyReader, respFlusher, buildConfig.RemoteURL, size)
	if err != nil {
		return err
	}
	defer func() {
		if err := context.Close(); err != nil {
			logrus.Debugf("[BUILDER] failed to remove temporary context: %v", err)
		}
	}()

	uidMaps, gidMaps := daemon.GetUIDGIDMaps()
	defaultArchiver := &archive.Archiver{
		Untar:   chrootarchive.Untar,
		UIDMaps: uidMaps,
		GIDMaps: gidMaps,
	}

	docker := Docker{daemon, respFlusher, authConfigs, defaultArchiver}

	b, err := dockerfile.NewBuilder(buildConfig, docker, builder.DockerIgnoreContext{context}, nil)
	if err != nil {
		return err
	}
	formattedOut := streamformatter.NewStdoutJSONFormattedWriter(respFlusher)

	b.Stdout = formattedOut
	b.Stderr = streamformatter.NewStderrJSONFormattedWriter(respFlusher)

	if closeNotifier, ok := respWriter.(http.CloseNotifier); ok {
		finished := make(chan struct{})
		defer close(finished)
		go func() {
			select {
			case <-finished:
			case <-closeNotifier.CloseNotify():
				logrus.Info("Client disconnected, cancelling job: build")
				b.Cancel()
			}
		}()
	}

	if len(dockerfileName) > 0 {
		b.DockerfileName = dockerfileName
	}

	imgID, err := b.Build()
	if err != nil {
		if !respFlusher.Flushed() {
			return err
		}

		fErr := errors.New(utils.GetErrorMessage(err))
		formattedOut.WriteError(fErr)
		return nil
	}

	for _, r := range buildConfig.ReposAndTags {
		if err := daemon.TagImage(r.Repo, r.Tag, imgID, true); err != nil {
			if !respFlusher.Flushed() {
				return err
			}

			fErr := errors.New(utils.GetErrorMessage(err))
			formattedOut.WriteError(fErr)
			return nil
		}
	}

	return nil
}
