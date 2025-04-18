package containerd

import (
	"context"
	"fmt"
	"io"
	"time"

	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/transfer"
	"github.com/containerd/containerd/v2/core/transfer/image"
	"github.com/containerd/containerd/v2/core/transfer/registry"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/metrics"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	dockerregistry "github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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

func (i *ImageService) pullTag(ctx context.Context, ref reference.Named, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registrytypes.AuthConfig, out progress.Output) (retErr error) {
	var oldImage, newImage *c8dimages.Image
	if img, err := i.images.Get(ctx, ref.String()); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return err
		}
	} else {
		oldImage = &img

		// Handle dangling images
		err = i.leaseContent(ctx, i.content, img.Target)
		if err != nil {
			return errdefs.System(fmt.Errorf("failed to lease content: %w", err))
		}
		defer func() {
			if retErr != nil {
				return
			}

			// Delete the dangling image if it exists.
			err := i.images.Delete(context.WithoutCancel(ctx), danglingImageName(oldImage.Target.Digest))
			if err != nil && !cerrdefs.IsNotFound(err) {
				log.G(ctx).WithError(err).Warn("unexpected error while removing outdated dangling image reference")
			}

			// If the pulled image is different than the old image, we will keep the old image as a dangling image.
			if newImage != nil && newImage.Target.Digest != oldImage.Target.Digest {
				if err := i.ensureDanglingImage(ctx, *oldImage); err != nil {
					log.G(ctx).WithError(err).Warn("failed to keep the previous image as dangling")
				}
			}
		}()
	}

	allPlatforms := false
	var sopts []image.StoreOpt

	if platform == nil {
		p := platforms.DefaultSpec()
		platform = &p
	}

	sopts = append(sopts, image.WithUnpack(*platform, i.snapshotter))
	if !allPlatforms {
		sopts = append(sopts, image.WithPlatforms(*platform))
	}

	// Temporariy disabled for consistency with pre-transfer service pull
	// sopts = append(sopts, image.WithAllMetadata)

	reg, err := registry.NewOCIRegistry(ctx, ref.String(),
		registry.WithHeaders(metaHeaders),
		registry.WithCredentials(authConfigCreds{authConfig}),
	)
	if err != nil {
		return err
	}
	is := image.NewStore(ref.String(), sopts...)

	pr := &transferProgress{
		tracked: map[digest.Digest]struct{}{},

		// TODO: Show extract progress
		// Needs a separate goroutine and synchronizing with the transfer progress
		// writing to the out stream.
		// Also needs https://github.com/containerd/containerd/pull/11195
		extract: &extractProgress{
			snapshotter: i.snapshotterService(i.snapshotter),
		},
	}

	topts := []transfer.Opt{
		transfer.WithProgress(func(p transfer.Progress) {
			pr.TransferProgress(ctx, out, p)
		}),
	}

	if err := i.transfer.Transfer(ctx, reg, is, topts...); err != nil {
		return err
	}

	if i, err := i.images.Get(ctx, ref.String()); err == nil {
		newImage = &i
	}

	// Print the new image digest and status message.
	progress.Message(out, "", "Digest: "+newImage.Target.Digest.String())
	writeStatus(out, reference.FamiliarString(ref), oldImage == nil || oldImage.Target.Digest != newImage.Target.Digest)

	i.LogImageEvent(ctx, reference.FamiliarString(ref), reference.FamiliarName(ref), events.ActionPull)
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

type authConfigCreds struct {
	*registrytypes.AuthConfig
}

func (c authConfigCreds) GetCredentials(ctx context.Context, refString, host string) (registry.Credentials, error) {
	if host == dockerregistry.DefaultRegistryHost {
		host = dockerregistry.IndexHostname
	}

	cfgHost := dockerregistry.ConvertToHostname(c.ServerAddress)
	if cfgHost == "" {
		if ref, err := reference.ParseNormalizedNamed(refString); err != nil {
			cfgHost = refString
		} else {
			cfgHost = reference.Domain(ref)
		}
	}
	if cfgHost == dockerregistry.IndexName || cfgHost == dockerregistry.DefaultRegistryHost {
		cfgHost = dockerregistry.IndexHostname
	}

	if host == cfgHost {
		if c.RegistryToken != "" {
			return registry.Credentials{
				Host:   c.ServerAddress,
				Header: "Bearer " + c.RegistryToken,
			}, nil
		}
		if c.IdentityToken != "" {
			return registry.Credentials{
				Host:   c.ServerAddress,
				Secret: c.IdentityToken,
			}, nil
		}
		if c.Password != "" {
			return registry.Credentials{
				Host:     c.ServerAddress,
				Username: c.Username,
				Secret:   c.Password,
			}, nil
		}
	}

	return registry.Credentials{}, nil
}
