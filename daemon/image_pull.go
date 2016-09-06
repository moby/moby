package daemon

import (
	"io"
	"strings"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"golang.org/x/net/context"
)

// PullImage initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func (daemon *Daemon) PullImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	// Special case: "pull -a" may send an image name with a
	// trailing :. This is ugly, but let's not break API
	// compatibility.
	image = strings.TrimSuffix(image, ":")

	ref, err := reference.ParseNamed(image)
	if err != nil {
		return err
	}

	if tag != "" {
		// The "tag" could actually be a digest.
		var dgst digest.Digest
		dgst, err = digest.ParseDigest(tag)
		if err == nil {
			ref, err = reference.WithDigest(ref, dgst)
		} else {
			ref, err = reference.WithTag(ref, tag)
		}
		if err != nil {
			return err
		}
	}

	return daemon.pullImageWithReference(ctx, ref, metaHeaders, authConfig, outStream)
}

// PullOnBuild tells Docker to pull image referenced by `name`.
func (daemon *Daemon) PullOnBuild(ctx context.Context, name string, authConfigs map[string]types.AuthConfig, output io.Writer) (builder.Image, error) {
	ref, err := reference.ParseNamed(name)
	if err != nil {
		return nil, err
	}
	ref = reference.WithDefaultTag(ref)

	pullRegistryAuth := &types.AuthConfig{}
	if len(authConfigs) > 0 {
		// The request came with a full auth config file, we prefer to use that
		repoInfo, err := daemon.RegistryService.ResolveRepository(ref)
		if err != nil {
			return nil, err
		}

		resolvedConfig := registry.ResolveAuthConfig(
			authConfigs,
			repoInfo.Index,
		)
		pullRegistryAuth = &resolvedConfig
	}

	if err := daemon.pullImageWithReference(ctx, ref, nil, pullRegistryAuth, output); err != nil {
		return nil, err
	}
	return daemon.GetImage(name)
}

func (daemon *Daemon) pullImageWithReference(ctx context.Context, ref reference.Named, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	// Include a buffer so that slow client connections don't affect
	// transfer performance.
	progressChan := make(chan progress.Progress, 100)

	writesDone := make(chan struct{})

	ctx, cancelFunc := context.WithCancel(ctx)

	go func() {
		writeDistributionProgress(cancelFunc, outStream, progressChan)
		close(writesDone)
	}()

	imagePullConfig := &distribution.ImagePullConfig{
		MetaHeaders:      metaHeaders,
		AuthConfig:       authConfig,
		ProgressOutput:   progress.ChanOutput(progressChan),
		RegistryService:  daemon.RegistryService,
		ImageEventLogger: daemon.LogImageEvent,
		MetadataStore:    daemon.distributionMetadataStore,
		ImageStore:       daemon.imageStore,
		ReferenceStore:   daemon.referenceStore,
		DownloadManager:  daemon.downloadManager,
	}

	err := distribution.Pull(ctx, ref, imagePullConfig)
	close(progressChan)
	<-writesDone
	return err
}
