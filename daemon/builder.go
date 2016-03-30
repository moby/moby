package daemon

import (
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

// BuildFromContext builds a new image from a given context.
func (daemon *Daemon) BuildFromContext(ctx context.Context, src io.ReadCloser, remote string, buildOptions *types.ImageBuildOptions, pg backend.ProgressWriter) (string, error) {
	buildContext, dockerfileName, err := builder.DetectContextFromRemoteURL(src, remote, pg.ProgressReaderFunc)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := buildContext.Close(); err != nil {
			logrus.Debugf("[BUILDER] failed to remove temporary context: %v", err)
		}
	}()
	if len(dockerfileName) > 0 {
		buildOptions.Dockerfile = dockerfileName
	}

	m := dockerfile.NewBuildManager(daemon)
	return m.Build(ctx, buildOptions,
		builder.DockerIgnoreContext{ModifiableContext: buildContext},
		pg.StdoutFormatter, pg.StderrFormatter, pg.Output)
}
