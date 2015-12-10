package daemon

import (
	"errors"
	"io"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/docker/daemon/daemonbuilder"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/utils"
)

// ImageBuild builds an image based in the given configuration.
// It generates a build context based in the input and prints progress in the output.
func (daemon *Daemon) ImageBuild(buildConfig *dockerfile.Config, input io.ReadCloser, output io.Writer) error {
	sf := streamformatter.NewJSONStreamFormatter()
	errf := func(err error) error {
		// Do not write the error in the http output if it's still empty.
		// This prevents from writing a 200(OK) when there is an interal error.
		if flusher, ok := output.(*ioutils.WriteFlusher); ok && !flusher.Flushed() {
			return err
		}
		_, err = output.Write(sf.FormatError(errors.New(utils.GetErrorMessage(err))))
		if err != nil {
			logrus.Warnf("could not write error response: %v", err)
		}
		return nil
	}
	// Currently, only used if context is from a remote url.
	// The field `In` is set by DetectContextFromRemoteURL.
	// Look at code in DetectContextFromRemoteURL for more information.
	pReader := &progressreader.Config{
		// TODO: make progressreader streamformatter-agnostic
		Out:       output,
		Formatter: sf,
		Size:      buildConfig.Size,
		NewLines:  true,
		ID:        "Downloading context",
		Action:    buildConfig.RemoteURL,
	}

	context, dockerfileName, err := daemonbuilder.DetectContextFromRemoteURL(input, buildConfig.RemoteURL, pReader)
	if err != nil {
		return errf(err)
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
	docker := &daemonbuilder.Docker{
		Daemon:      daemon,
		OutOld:      output,
		AuthConfigs: buildConfig.AuthConfigs,
		Archiver:    defaultArchiver,
	}

	b, err := dockerfile.NewBuilder(buildConfig, docker, builder.DockerIgnoreContext{ModifiableContext: context}, nil)
	if err != nil {
		return errf(err)
	}
	b.Stdout = &streamformatter.StdoutFormatter{Writer: output, StreamFormatter: sf}
	b.Stderr = &streamformatter.StderrFormatter{Writer: output, StreamFormatter: sf}

	if closeNotifier, ok := output.(http.CloseNotifier); ok {
		finished := make(chan struct{})
		defer close(finished)
		go func() {
			select {
			case <-finished:
			case <-closeNotifier.CloseNotify():
				logrus.Infof("Client disconnected, cancelling job: build")
				b.Cancel()
			}
		}()
	}

	if len(dockerfileName) > 0 {
		b.DockerfileName = dockerfileName
	}

	imgID, err := b.Build()
	if err != nil {
		return errf(err)
	}

	for _, rt := range buildConfig.ReposAndTags {
		if err := daemon.TagImage(rt, imgID); err != nil {
			return errf(err)
		}
	}

	return nil
}
