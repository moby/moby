package images

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	librsync "github.com/balena-os/librsync-go"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/daemon/internal/distribution"
	progressutils "github.com/moby/moby/v2/daemon/internal/distribution/utils"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/moby/moby/v2/daemon/internal/metrics"
	"github.com/moby/moby/v2/daemon/internal/progress"
	"github.com/moby/moby/v2/daemon/internal/streamformatter"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// PullImage initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func (i *ImageService) PullImage(ctx context.Context, ref reference.Named, options imagebackend.PullOptions) error {
	if len(options.Platforms) > 1 {
		// TODO(thaJeztah): add support for pulling multiple platforms
		return cerrdefs.ErrInvalidArgument.WithMessage("multiple platforms is not supported")
	}
	start := time.Now()

	var platform *ocispec.Platform
	if len(options.Platforms) > 0 {
		p := options.Platforms[0]
		platform = &p
	}

	err := i.pullImageWithReference(ctx, ref, platform, options.MetaHeaders, options.AuthConfig, options.OutStream)
	metrics.ImageActions.WithValues("pull").UpdateSince(start)
	if err != nil {
		return err
	}

	// Check if the pulled image is a delta and apply it if needed
	if err := i.applyDeltaIfNeeded(ctx, ref, options.OutStream); err != nil {
		return errors.Wrap(err, "failed to apply delta image")
	}

	if platform != nil {
		// If --platform was specified, check that the image we pulled matches
		// the expected platform. This check is for situations where the image
		// is a single-arch image, in which case (for backward compatibility),
		// we allow the image to have a non-matching architecture. The code
		// below checks for this situation, and returns a warning to the client,
		// as well as logging it to the daemon logs.
		img, err := i.GetImage(ctx, ref.String(), imagebackend.GetImageOpts{Platform: platform})

		// Note that this is a special case where GetImage returns both an image
		// and an error: https://github.com/moby/moby/blob/v28.3.3/daemon/images/image.go#L186-L193
		if cerrdefs.IsNotFound(err) && img != nil {
			po := streamformatter.NewJSONProgressOutput(options.OutStream, false)
			progress.Messagef(po, "", `WARNING: %s`, err.Error())
			log.G(ctx).WithError(err).WithField("image", reference.FamiliarName(ref)).Warn("ignoring platform mismatch on single-arch image")
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (i *ImageService) pullImageWithReference(ctx context.Context, ref reference.Named, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	// Include a buffer so that slow client connections don't affect
	// transfer performance.
	progressChan := make(chan progress.Progress, 100)

	writesDone := make(chan struct{})

	ctx, cancelFunc := context.WithCancel(ctx)

	go func() {
		progressutils.WriteDistributionProgress(cancelFunc, outStream, progressChan)
		close(writesDone)
	}()

	ctx = namespaces.WithNamespace(ctx, i.contentNamespace)
	// Take out a temporary lease for everything that gets persisted to the content store.
	// Before the lease is cancelled, any content we want to keep should have it's own lease applied.
	ctx, done, err := tempLease(ctx, i.leases)
	if err != nil {
		return err
	}
	defer done(ctx)

	cs := &contentStoreForPull{
		ContentStore: i.content,
		leases:       i.leases,
	}
	imageStore := &imageStoreForPull{
		ImageConfigStore: distribution.NewImageConfigStoreFromStore(i.imageStore),
		ingested:         cs,
		leases:           i.leases,
	}

	imagePullConfig := &distribution.ImagePullConfig{
		Config: distribution.Config{
			MetaHeaders:      metaHeaders,
			AuthConfig:       authConfig,
			ProgressOutput:   progress.ChanOutput(progressChan),
			RegistryService:  i.registryService,
			ImageEventLogger: i.LogImageEvent,
			MetadataStore:    i.distributionMetadataStore,
			ImageStore:       imageStore,
			ReferenceStore:   i.referenceStore,
		},
		DownloadManager: i.downloadManager,
		Platform:        platform,
	}

	err = distribution.Pull(ctx, ref, imagePullConfig, cs)
	close(progressChan)
	<-writesDone
	return err
}

func tempLease(ctx context.Context, mgr leases.Manager) (context.Context, func(context.Context) error, error) {
	nop := func(context.Context) error { return nil }
	_, ok := leases.FromContext(ctx)
	if ok {
		return ctx, nop, nil
	}

	// Use an expiration that ensures the lease is cleaned up at some point if there is a crash, SIGKILL, etc.
	opts := []leases.Opt{
		leases.WithRandomID(),
		leases.WithExpiration(24 * time.Hour),
		leases.WithLabels(map[string]string{
			"moby.lease/temporary": time.Now().UTC().Format(time.RFC3339Nano),
		}),
	}
	l, err := mgr.Create(ctx, opts...)
	if err != nil {
		return ctx, nop, errors.Wrap(err, "error creating temporary lease")
	}

	ctx = leases.WithLease(ctx, l.ID)
	return ctx, func(ctx context.Context) error {
		return mgr.Delete(ctx, l)
	}, nil
}

// applyDeltaIfNeeded checks if the pulled image is a delta image and applies it to reconstruct the target image.
// Delta images are identified by the presence of the DeltaBaseLabel in their configuration.
func (i *ImageService) applyDeltaIfNeeded(ctx context.Context, ref reference.Named, outStream io.Writer) error {
	// Get the pulled image
	img, err := i.GetImage(ctx, ref.String(), imagebackend.GetImageOpts{})
	if err != nil {
		return errors.Wrap(err, "failed to get pulled image")
	}

	// Check if it's a delta image
	if !isDeltaImage(img) {
		return nil // Not a delta, nothing to do
	}

	progressOutput := streamformatter.NewJSONProgressOutput(outStream, false)
	progress.Messagef(progressOutput, "", "Delta image detected, applying to reconstruct target image")

	// Get the base image ID from delta labels
	baseID, err := getBaseImageID(img)
	if err != nil {
		return errors.Wrap(err, "failed to get base image ID from delta")
	}

	// Check if base image exists locally
	_, err = i.GetImage(ctx, baseID.String(), imagebackend.GetImageOpts{})
	if err != nil {
		return errors.Wrapf(err, "delta requires base image %s which is not present locally", baseID)
	}

	progress.Messagef(progressOutput, "", "Base image %s found, applying delta", baseID)

	// Export base and delta images to reconstruct the target
	progress.Messagef(progressOutput, "", "Exporting base image")
	baseTar := new(bytes.Buffer)
	if err := i.ExportImage(ctx, []string{baseID.String()}, nil, baseTar); err != nil {
		return errors.Wrap(err, "failed to export base image")
	}

	// Extract delta data from the pulled delta image
	progress.Messagef(progressOutput, "", "Extracting delta data")
	deltaData, err := i.extractDeltaFromImage(ctx, img)
	if err != nil {
		return errors.Wrap(err, "failed to extract delta data")
	}

	// Apply librsync patch to reconstruct target
	progress.Messagef(progressOutput, "", "Applying delta patch")
	targetTar := new(bytes.Buffer)
	if err := librsync.Patch(bytes.NewReader(baseTar.Bytes()), bytes.NewReader(deltaData), targetTar); err != nil {
		return errors.Wrap(err, "failed to apply delta patch")
	}

	// Load the reconstructed target image
	progress.Messagef(progressOutput, "", "Loading reconstructed target image")
	if err := i.LoadImage(ctx, io.NopCloser(targetTar), nil, io.Discard, true); err != nil {
		return errors.Wrap(err, "failed to load reconstructed target image")
	}

	// Get target image ID from delta config
	cfg, err := getDeltaConfig(img)
	if err == nil && cfg.TargetID != "" {
		progress.Messagef(progressOutput, "", "Delta applied successfully, reconstructed image: %s", cfg.TargetID)
	} else {
		progress.Messagef(progressOutput, "", "Delta applied successfully")
	}

	// Clean up the delta image as it's no longer needed
	// (Optional - the delta image remains available)

	return nil
}

// extractDeltaFromImage extracts delta data from a delta image
func (i *ImageService) extractDeltaFromImage(ctx context.Context, img *image.Image) ([]byte, error) {
	if len(img.RootFS.DiffIDs) == 0 {
		return nil, errors.New("delta image has no layers")
	}

	// Get the delta layer
	deltaLayer, err := i.layerStore.Get(img.RootFS.ChainID())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get delta layer")
	}
	defer layer.ReleaseAndLog(i.layerStore, deltaLayer)

	// Get tar stream of the layer
	tarStream, err := deltaLayer.TarStream()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tar stream")
	}
	defer tarStream.Close()

	// Read through tar to find delta.bin
	tr := tar.NewReader(tarStream)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read tar header")
		}

		if header.Name == "delta.bin" {
			deltaData, err := io.ReadAll(tr)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read delta.bin")
			}
			return deltaData, nil
		}
	}

	return nil, errors.New("delta.bin not found in delta image layer")
}

// getDeltaConfig extracts and parses the delta configuration from labels
func getDeltaConfig(img *image.Image) (*deltaConfig, error) {
	if img.Config == nil {
		return nil, errors.New("image config is nil")
	}
	configStr, ok := img.Config.Labels[DeltaConfigLabel]
	if !ok {
		return nil, errors.New("delta image missing config label")
	}

	var cfg deltaConfig
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return nil, errors.Wrap(err, "failed to parse delta config")
	}
	return &cfg, nil
}

// deltaConfig represents the metadata stored in the delta config label
type deltaConfig struct {
	DeltaSize  int    `json:"deltaSize"`
	TargetID   string `json:"targetID"`
	Compressed bool   `json:"compressed"`
}

// isDeltaImage checks if an image is a delta by looking for the delta base label
func isDeltaImage(img *image.Image) bool {
	if img.Config == nil {
		return false
	}
	_, hasBase := img.Config.Labels[DeltaBaseLabel]
	return hasBase
}

// getBaseImageID extracts the base image ID from a delta image's labels
func getBaseImageID(img *image.Image) (image.ID, error) {
	if img.Config == nil {
		return "", errors.New("image config is nil")
	}
	baseIDStr, ok := img.Config.Labels[DeltaBaseLabel]
	if !ok {
		return "", errors.New("delta image missing base label")
	}
	return image.ID(baseIDStr), nil
}
