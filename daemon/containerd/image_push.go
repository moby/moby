package containerd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

// PushImage initiates a push operation of the image pointed to by targetRef.
// Image manifest (or index) is pushed as is, which will probably fail if you
// don't have all content referenced by the index.
// Cross-repo mounts will be attempted for non-existing blobs.
//
// It will also add distribution source labels to the pushed content
// pointing to the new target repository. This will allow subsequent pushes
// to perform cross-repo mounts of the shared content when pushing to a different
// repository on the same registry.
func (i *ImageService) PushImage(ctx context.Context, targetRef reference.Named, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	if _, tagged := targetRef.(reference.Tagged); !tagged {
		if _, digested := targetRef.(reference.Digested); !digested {
			return errdefs.NotImplemented(errors.New("push all tags is not implemented"))
		}
	}

	leasedCtx, release, err := i.client.WithLease(ctx)
	if err != nil {
		return err
	}
	defer release(leasedCtx)
	out := streamformatter.NewJSONProgressOutput(outStream, false)

	img, err := i.client.ImageService().Get(ctx, targetRef.String())
	if err != nil {
		return errdefs.NotFound(err)
	}

	target := img.Target
	store := i.client.ContentStore()

	resolver, tracker := i.newResolverFromAuthConfig(authConfig)
	progress := pushProgress{Tracker: tracker}
	jobs := newJobs()
	finishProgress := jobs.showProgress(ctx, out, combinedProgress([]progressUpdater{
		&progress,
		pullProgress{ShowExists: false, Store: store},
	}))
	defer finishProgress()

	var limiter *semaphore.Weighted = nil // TODO: Respect max concurrent downloads/uploads

	mountableBlobs, err := i.findMissingMountable(ctx, store, jobs, target, targetRef, limiter)
	if err != nil {
		return err
	}
	for dgst := range mountableBlobs {
		progress.addMountable(dgst)
	}

	// Create a store which fakes the local existence of possibly mountable blobs.
	// Otherwise they can't be pushed at all.
	realStore := store
	wrapped := wrapWithFakeMountableBlobs(store, mountableBlobs)
	store = wrapped

	pusher, err := resolver.Pusher(ctx, targetRef.String())
	if err != nil {
		return err
	}

	addChildrenToJobs := containerdimages.HandlerFunc(
		func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			children, err := containerdimages.Children(ctx, store, desc)
			if err != nil {
				return nil, err
			}
			for _, c := range children {
				jobs.Add(c)
			}

			jobs.Add(desc)

			return nil, nil
		},
	)

	appendSource, err := docker.AppendDistributionSourceLabel(realStore, targetRef.String())
	if err != nil {
		// This shouldn't happen at this point because the reference would have to be invalid
		// and if it was, then it would error out earlier.
		return errdefs.Unknown(errors.Wrap(err, "failed to create an handler that appends distribution source label to pushed content"))
	}

	handlerWrapper := func(h images.Handler) images.Handler {
		return containerdimages.Handlers(addChildrenToJobs, h, appendSource)
	}

	err = remotes.PushContent(ctx, pusher, target, store, limiter, platforms.All, handlerWrapper)
	if err != nil {
		if containerdimages.IsIndexType(target.MediaType) {
			if cerrdefs.IsNotFound(err) {
				err = errdefs.NotFound(fmt.Errorf(
					"missing content: %w\n"+
						"Note: You're trying to push a manifest list/index which "+
						"references multiple platform specific manifests, but not all of them are available locally "+
						"or available to the remote repository.\n"+
						"Make sure you have all the referenced content and try again.",
					err))
			}
		}
	}
	return err
}

// findMissingMountable will walk the target descriptor recursively and return
// missing contents with their distribution source which could potentially
// be cross-repo mounted.
func (i *ImageService) findMissingMountable(ctx context.Context, store content.Store, jobs *jobs,
	target ocispec.Descriptor, targetRef reference.Named, limiter *semaphore.Weighted,
) (map[digest.Digest]distributionSource, error) {
	mountableBlobs := map[digest.Digest]distributionSource{}
	sources, err := getDigestSources(ctx, store, target.Digest)

	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		logrus.WithField("target", target).Debug("distribution source label not found")
		return mountableBlobs, nil
	}

	handler := func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		_, err := store.Info(ctx, desc.Digest)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				return nil, errdefs.System(errors.Wrapf(err, "failed to get metadata of content %s", desc.Digest.String()))
			}

			for _, source := range sources {
				if canBeMounted(desc.MediaType, targetRef, i.registryService.IsInsecureRegistry, source) {
					mountableBlobs[desc.Digest] = source
					jobs.Add(desc)
					break
				}
			}
			return nil, nil
		}

		return containerdimages.Children(ctx, store, desc)
	}

	err = containerdimages.Dispatch(ctx, containerdimages.HandlerFunc(handler), limiter, target)
	if err != nil {
		return nil, err
	}

	return mountableBlobs, nil
}

func getDigestSources(ctx context.Context, store content.Manager, digest digest.Digest) ([]distributionSource, error) {
	info, err := store.Info(ctx, digest)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, errdefs.NotFound(err)
		}
		return nil, errdefs.System(err)
	}

	sources := extractDistributionSources(info.Labels)
	if sources == nil {
		return nil, errdefs.NotFound(fmt.Errorf("label %q is not attached to %s", labelDistributionSource, digest.String()))
	}

	return sources, nil
}

// TODO(vvoland): Remove and use containerd const in containerd 1.7+
// https://github.com/containerd/containerd/pull/8224
const labelDistributionSource = "containerd.io/distribution.source."

func extractDistributionSources(labels map[string]string) []distributionSource {
	var sources []distributionSource

	// Check if this blob has a distributionSource label
	// if yes, read it as source
	for k, v := range labels {
		if reg := strings.TrimPrefix(k, labelDistributionSource); reg != k {
			ref, err := reference.ParseNamed(reg + "/" + v)
			if err != nil {
				continue
			}

			sources = append(sources, distributionSource{
				registryRef: ref,
			})
		}
	}

	return sources
}

type distributionSource struct {
	registryRef reference.Named
}

// ToAnnotation returns key and value
func (source distributionSource) ToAnnotation() (string, string) {
	domain := reference.Domain(source.registryRef)
	v := reference.Path(source.registryRef)
	return labelDistributionSource + domain, v
}

func (source distributionSource) GetReference(dgst digest.Digest) (reference.Named, error) {
	return reference.WithDigest(source.registryRef, dgst)
}

// canBeMounted returns if the content with given media type can be cross-repo
// mounted when pushing it to a remote reference ref.
func canBeMounted(mediaType string, targetRef reference.Named, isInsecureFunc func(string) bool, source distributionSource) bool {
	if containerdimages.IsManifestType(mediaType) {
		return false
	}
	if containerdimages.IsIndexType(mediaType) {
		return false
	}

	reg := reference.Domain(targetRef)

	// Cross-repo mount doesn't seem to work with insecure registries.
	isInsecure := isInsecureFunc(reg)
	if isInsecure {
		return false
	}

	// If the source registry is the same as the one we are pushing to
	// then the cross-repo mount will work.
	return reg == reference.Domain(source.registryRef)
}
