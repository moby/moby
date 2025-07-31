package images

import (
	"context"
	"io"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/moby/moby/api/pkg/progress"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/daemon/internal/distribution"
	progressutils "github.com/moby/moby/v2/daemon/internal/distribution/utils"
	"github.com/moby/moby/v2/daemon/internal/metrics"
	"github.com/moby/moby/v2/daemon/server/backend"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// PushImage initiates a push operation on the repository named localName.
func (i *ImageService) PushImage(ctx context.Context, ref reference.Named, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	if platform != nil {
		// Check if the image is actually the platform we want to push.
		_, err := i.GetImage(ctx, ref.String(), backend.GetImageOpts{Platform: platform})
		if err != nil {
			return err
		}
	}
	start := time.Now()
	// Include a buffer so that slow client connections don't affect
	// transfer performance.
	progressChan := make(chan progress.Progress, 100)

	writesDone := make(chan struct{})

	ctx, cancelFunc := context.WithCancel(ctx)

	go func() {
		progressutils.WriteDistributionProgress(cancelFunc, outStream, progressChan)
		close(writesDone)
	}()

	imagePushConfig := &distribution.ImagePushConfig{
		Config: distribution.Config{
			MetaHeaders:      metaHeaders,
			AuthConfig:       authConfig,
			ProgressOutput:   progress.ChanOutput(progressChan),
			RegistryService:  i.registryService,
			ImageEventLogger: i.LogImageEvent,
			MetadataStore:    i.distributionMetadataStore,
			ImageStore:       distribution.NewImageConfigStoreFromStore(i.imageStore),
			ReferenceStore:   i.referenceStore,
		},
		ConfigMediaType: schema2.MediaTypeImageConfig,
		LayerStores:     distribution.NewLayerProvidersFromStore(i.layerStore),
		UploadManager:   i.uploadManager,
	}

	err := distribution.Push(ctx, ref, imagePushConfig)
	close(progressChan)
	<-writesDone
	metrics.ImageActions.WithValues("push").UpdateSince(start)
	return err
}
