package containerd

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/pkg/snapshotters"
	"github.com/containerd/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// PullImage initiates a pull operation. image is the repository name to pull, and
// tagOrDigest may be either empty, or indicate a specific tag or digest to pull.
func (i *ImageService) PullImage(ctx context.Context, image, tagOrDigest string, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	var opts []containerd.RemoteOpt
	if platform != nil {
		opts = append(opts, containerd.WithPlatform(platforms.Format(*platform)))
	}
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return errdefs.InvalidParameter(err)
	}

	// TODO(thaJeztah) this could use a WithTagOrDigest() utility
	if tagOrDigest != "" {
		// The "tag" could actually be a digest.
		var dgst digest.Digest
		dgst, err = digest.Parse(tagOrDigest)
		if err == nil {
			ref, err = reference.WithDigest(reference.TrimNamed(ref), dgst)
		} else {
			ref, err = reference.WithTag(ref, tagOrDigest)
		}
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
	}

	resolver, _ := i.newResolverFromAuthConfig(ctx, authConfig)
	opts = append(opts, containerd.WithResolver(resolver))

	old, err := i.resolveDescriptor(ctx, ref.String())
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	p := platforms.Default()
	if platform != nil {
		p = platforms.Only(*platform)
	}

	jobs := newJobs()
	h := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if images.IsLayerType(desc.MediaType) {
			jobs.Add(desc)
		}
		return nil, nil
	})
	opts = append(opts, containerd.WithImageHandler(h))

	out := streamformatter.NewJSONProgressOutput(outStream, false)
	pp := pullProgress{store: i.client.ContentStore(), showExists: true}
	finishProgress := jobs.showProgress(ctx, out, pp)
	defer finishProgress()

	ah := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if images.IsManifestType(desc.MediaType) {
			available, _, _, missing, err := images.Check(ctx, i.client.ContentStore(), desc, p)
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
	infoHandler := snapshotters.AppendInfoHandlerWrapper(ref.String())
	opts = append(opts, containerd.WithImageHandlerWrapper(infoHandler))

	progress.Message(out, tagOrDigest, "Pulling from "+reference.Path(ref))

	img, err := i.client.Pull(ctx, ref.String(), opts...)
	if err != nil {
		return err
	}

	logger := log.G(ctx).WithFields(log.Fields{
		"digest": img.Target().Digest,
		"remote": ref.String(),
	})
	logger.Info("image pulled")
	progress.Message(out, "", "Digest: "+img.Target().Digest.String())

	writeStatus(out, reference.FamiliarString(ref), old.Digest != img.Target().Digest)

	// The pull succeeded, so try to remove any dangling image we have for this target
	err = i.client.ImageService().Delete(context.Background(), danglingImageName(img.Target().Digest))
	if err != nil && !cerrdefs.IsNotFound(err) {
		// Image pull succeeded, but cleaning up the dangling image failed. Ignore the
		// error to not mark the pull as failed.
		logger.WithError(err).Warn("unexpected error while removing outdated dangling image reference")
	}

	i.LogImageEvent(reference.FamiliarString(ref), reference.FamiliarName(ref), events.ActionPull)

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
