package containerd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/snapshotters"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/moby/moby/api/pkg/progress"
	"github.com/moby/moby/api/pkg/streamformatter"
	"github.com/moby/moby/api/types/events"
	registrytypes "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/daemon/internal/distribution"
	"github.com/moby/moby/v2/daemon/internal/metrics"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/moby/v2/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// PullImage initiates a pull operation. baseRef is the image to pull.
// If reference is not tagged, all tags are pulled.
func (i *ImageService) PullImage(ctx context.Context, baseRef reference.Named, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registrytypes.AuthConfig, outStream io.Writer) (retErr error) {
	start := time.Now()
	defer func() {
		if retErr == nil {
			metrics.ImageActions.WithValues("pull").UpdateSince(start)
		}
	}()
	out := streamformatter.NewJSONProgressOutput(outStream, false)

	ctx, done, err := i.withLease(ctx, true)
	if err != nil {
		return err
	}
	defer done()

	if !reference.IsNameOnly(baseRef) {
		return i.pullTag(ctx, baseRef, platform, metaHeaders, authConfig, out)
	}

	tags, err := distribution.Tags(ctx, baseRef, &distribution.Config{
		RegistryService: i.registryService,
		MetaHeaders:     metaHeaders,
		AuthConfig:      authConfig,
	})
	if err != nil {
		return err
	}

	for _, tag := range tags {
		ref, err := reference.WithTag(baseRef, tag)
		if err != nil {
			log.G(ctx).WithFields(log.Fields{
				"tag":     tag,
				"baseRef": baseRef,
			}).Warn("invalid tag, won't pull")
			continue
		}

		if err := i.pullTag(ctx, ref, platform, metaHeaders, authConfig, out); err != nil {
			return fmt.Errorf("error pulling %s: %w", ref, err)
		}
	}

	return nil
}

func (i *ImageService) pullTag(ctx context.Context, ref reference.Named, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registrytypes.AuthConfig, out progress.Output) error {
	var opts []containerd.RemoteOpt
	if platform != nil {
		opts = append(opts, containerd.WithPlatform(platforms.FormatAll(*platform)))
	}

	resolver, _ := i.newResolverFromAuthConfig(ctx, authConfig, ref)
	opts = append(opts, containerd.WithResolver(resolver))

	oldImage, err := i.resolveImage(ctx, ref.String())
	if err != nil && !cerrdefs.IsNotFound(err) {
		return err
	}

	// Will be set to the new image after pull succeeds.
	var outNewImg containerd.Image

	if oldImage.Target.Digest != "" {
		err = i.leaseContent(ctx, i.content, oldImage.Target)
		if err != nil {
			return errdefs.System(fmt.Errorf("failed to lease content: %w", err))
		}

		// If the pulled image is different than the old image, we will keep the old image as a dangling image.
		defer func() {
			if outNewImg != nil {
				if outNewImg.Target().Digest != oldImage.Target.Digest {
					if err := i.ensureDanglingImage(ctx, oldImage); err != nil {
						log.G(ctx).WithError(err).Warn("failed to keep the previous image as dangling")
					}
				}
			}
		}()
	}

	p := platforms.Default()
	if platform != nil {
		p = platforms.Only(*platform)
	}

	pullJobs := newJobs()
	opts = append(opts, containerd.WithImageHandler(c8dimages.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if showBlobProgress(desc) {
			pullJobs.Add(desc)
		}
		return nil, nil
	})))

	pp := &pullProgress{
		store:       i.content,
		snapshotter: i.snapshotterService(i.snapshotter),
		showExists:  true,
	}
	finishProgress := pullJobs.showProgress(ctx, out, pp)

	defer func() {
		finishProgress()

		// Send final status message after the progress updater has finished.
		// Otherwise the layer/manifest progress messages may arrive AFTER the
		// status message have been sent, so they won't update the previous
		// progress leaving stale progress like:
		// 70f5ac315c5a: Downloading [>       ]       0B/3.19kB
		// Digest: sha256:4f53e2564790c8e7856ec08e384732aa38dc43c52f02952483e3f003afbf23db
		// 70f5ac315c5a: Download complete
		// Status: Downloaded newer image for hello-world:latest
		// docker.io/library/hello-world:latest
		if outNewImg != nil {
			img := outNewImg
			progress.Message(out, "", "Digest: "+img.Target().Digest.String())
			newer := oldImage.Target.Digest != img.Target().Digest
			writeStatus(out, reference.FamiliarString(ref), newer)
		}
	}()

	var sentPullingFrom, sentModelNotSupported atomic.Bool
	ah := c8dimages.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.MediaType == c8dimages.MediaTypeDockerSchema1Manifest {
			return nil, distribution.DeprecatedSchema1ImageError(ref)
		}

		ociAiArtifactManifest := c8dimages.IsManifestType(desc.MediaType) && isModelMediaType(desc.ArtifactType)
		aiMediaType := isModelMediaType(desc.MediaType)

		if ociAiArtifactManifest || aiMediaType {
			if !sentModelNotSupported.Load() {
				sentModelNotSupported.Store(true)
				progress.Message(out, "", `WARNING: AI models are not supported by the Engine yet, did you mean to use "docker model pull/run" instead?`)
			}
		}
		if c8dimages.IsLayerType(desc.MediaType) {
			id := stringid.TruncateID(desc.Digest.String())
			progress.Update(out, id, "Pulling fs layer")
		}
		if c8dimages.IsManifestType(desc.MediaType) {
			if !sentPullingFrom.Load() {
				var tagOrDigest string
				if tagged, ok := ref.(reference.Tagged); ok {
					tagOrDigest = tagged.Tag()
				} else {
					tagOrDigest = ref.String()
				}
				progress.Message(out, tagOrDigest, "Pulling from "+reference.Path(ref))
				sentPullingFrom.Store(true)
			}

			available, _, _, missing, err := c8dimages.Check(ctx, i.content, desc, p)
			if err != nil {
				return nil, err
			}
			// If we already have all the contents pull shouldn't show any layer
			// download progress, not even a "Already present" message.
			if available && len(missing) == 0 {
				pp.hideLayers = true
			}
		}
		return nil, nil
	})
	opts = append(opts, containerd.WithImageHandler(ah))

	opts = append(opts, containerd.WithPullUnpack)
	// TODO(thaJeztah): we may have to pass the snapshotter to use if the pull is part of a "docker run" (container create -> pull image if missing). See https://github.com/moby/moby/issues/45273
	opts = append(opts, containerd.WithPullSnapshotter(i.snapshotter))

	// AppendInfoHandlerWrapper will annotate the image with basic information like manifest and layer digests as labels;
	// this information is used to enable remote snapshotters like nydus and stargz to query a registry.
	// This is also needed for the pull progress to detect the `Extracting` status.
	infoHandler := snapshotters.AppendInfoHandlerWrapper(ref.String())
	opts = append(opts, containerd.WithImageHandlerWrapper(infoHandler))

	img, err := i.client.Pull(ctx, ref.String(), opts...)
	if err != nil {
		if errors.Is(err, docker.ErrInvalidAuthorization) {
			// Match error returned by containerd.
			// https://github.com/containerd/containerd/blob/v2.1.1/core/remotes/docker/authorizer.go#L201-L203
			if strings.Contains(err.Error(), "no basic auth credentials") {
				return err
			}
			return errdefs.NotFound(fmt.Errorf("pull access denied for %s, repository does not exist or may require 'docker login'", reference.FamiliarName(ref)))
		}
		if cerrdefs.IsNotFound(err) {
			// Transform "no match for platform in manifest" error returned by containerd into
			// the same message as the graphdrivers backend.
			// The one returned by containerd doesn't contain the platform and is much less informative.
			if strings.Contains(err.Error(), "platform") {
				platformStr := platforms.DefaultString()
				if platform != nil {
					platformStr = platforms.FormatAll(*platform)
				}
				return errdefs.NotFound(fmt.Errorf("no matching manifest for %s in the manifest list entries: %w", platformStr, err))
			}
		}
		return translateRegistryError(ctx, err)
	}

	logger := log.G(ctx).WithFields(log.Fields{
		"digest": img.Target().Digest,
		"remote": ref.String(),
	})
	logger.Info("image pulled")

	// The pull succeeded, so try to remove any dangling image we have for this target
	err = i.images.Delete(context.WithoutCancel(ctx), danglingImageName(img.Target().Digest))
	if err != nil && !cerrdefs.IsNotFound(err) {
		// Image pull succeeded, but cleaning up the dangling image failed. Ignore the
		// error to not mark the pull as failed.
		logger.WithError(err).Warn("unexpected error while removing outdated dangling image reference")
	}

	i.LogImageEvent(ctx, reference.FamiliarString(ref), reference.FamiliarName(ref), events.ActionPull)
	outNewImg = img
	return nil
}

// writeStatus writes a status message to out. If newerDownloaded is true, the
// status message indicates that a newer image was downloaded. Otherwise, it
// indicates that the image is up to date. requestedTag is the tag the message
// will refer to.
func writeStatus(out progress.Output, requestedTag string, newerDownloaded bool) {
	if newerDownloaded {
		progress.Message(out, "", "Status: Downloaded newer image for "+requestedTag)
	} else {
		progress.Message(out, "", "Status: Image is up to date for "+requestedTag)
	}
}

func isModelMediaType(mediaType string) bool {
	return strings.HasPrefix(strings.ToLower(mediaType), "application/vnd.docker.ai.")
}
