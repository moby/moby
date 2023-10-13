package containerd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	containerdimages "github.com/containerd/containerd/images"
	containerdlabels "github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/compatcontext"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

// PushImage initiates a push operation of the image pointed to by sourceRef.
// If reference is untagged, all tags from the reference repository are pushed.
// Image manifest (or index) is pushed as is, which will probably fail if you
// don't have all content referenced by the index.
// Cross-repo mounts will be attempted for non-existing blobs.
//
// It will also add distribution source labels to the pushed content
// pointing to the new target repository. This will allow subsequent pushes
// to perform cross-repo mounts of the shared content when pushing to a different
// repository on the same registry.
func (i *ImageService) PushImage(ctx context.Context, sourceRef reference.Named, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) (retErr error) {
	out := streamformatter.NewJSONProgressOutput(outStream, false)
	progress.Messagef(out, "", "The push refers to repository [%s]", sourceRef.Name())

	if _, tagged := sourceRef.(reference.Tagged); !tagged {
		if _, digested := sourceRef.(reference.Digested); !digested {
			// Image is not tagged nor digested, that means all tags push was requested.

			// Find all images with the same repository.
			imgs, err := i.getAllImagesWithRepository(ctx, sourceRef)
			if err != nil {
				return err
			}

			for _, img := range imgs {
				named, err := reference.ParseNamed(img.Name)
				if err != nil {
					// This shouldn't happen, but log a warning just in case.
					log.G(ctx).WithFields(log.Fields{
						"image":     img.Name,
						"sourceRef": sourceRef,
					}).Warn("refusing to push an invalid tag")
					continue
				}

				if err := i.pushRef(ctx, named, metaHeaders, authConfig, out); err != nil {
					return err
				}
			}

			return nil
		}
	}

	return i.pushRef(ctx, sourceRef, metaHeaders, authConfig, out)
}

func (i *ImageService) pushRef(ctx context.Context, targetRef reference.Named, metaHeaders map[string][]string, authConfig *registry.AuthConfig, out progress.Output) (retErr error) {
	leasedCtx, release, err := i.client.WithLease(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := release(compatcontext.WithoutCancel(leasedCtx)); err != nil {
			log.G(ctx).WithField("image", targetRef).WithError(err).Warn("failed to release lease created for push")
		}
	}()

	img, err := i.client.ImageService().Get(ctx, targetRef.String())
	if err != nil {
		return errdefs.NotFound(err)
	}

	target := img.Target
	store := i.client.ContentStore()

	resolver, tracker := i.newResolverFromAuthConfig(ctx, authConfig)
	pp := pushProgress{Tracker: tracker}
	jobsQueue := newJobs()
	finishProgress := jobsQueue.showProgress(ctx, out, combinedProgress([]progressUpdater{
		&pp,
		pullProgress{showExists: false, store: store},
	}))
	defer func() {
		finishProgress()
		if retErr == nil {
			if tagged, ok := targetRef.(reference.Tagged); ok {
				progress.Messagef(out, "", "%s: digest: %s size: %d", tagged.Tag(), target.Digest, img.Target.Size)
			}
		}
	}()

	var limiter *semaphore.Weighted = nil // TODO: Respect max concurrent downloads/uploads

	mountableBlobs, err := findMissingMountable(ctx, store, jobsQueue, target, targetRef, limiter)
	if err != nil {
		return err
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
				jobsQueue.Add(c)
			}

			jobsQueue.Add(desc)

			return nil, nil
		},
	)

	handlerWrapper := func(h images.Handler) images.Handler {
		return containerdimages.Handlers(addChildrenToJobs, h)
	}

	err = remotes.PushContent(ctx, pusher, target, store, limiter, platforms.All, handlerWrapper)
	if err != nil {
		if containerdimages.IsIndexType(target.MediaType) && cerrdefs.IsNotFound(err) {
			return errdefs.NotFound(fmt.Errorf(
				"missing content: %w\n"+
					"Note: You're trying to push a manifest list/index which "+
					"references multiple platform specific manifests, but not all of them are available locally "+
					"or available to the remote repository.\n"+
					"Make sure you have all the referenced content and try again.",
				err))
		}
		return err
	}

	appendDistributionSourceLabel(ctx, realStore, targetRef, target)

	i.LogImageEvent(reference.FamiliarString(targetRef), reference.FamiliarName(targetRef), events.ActionPush)

	return nil
}

func appendDistributionSourceLabel(ctx context.Context, realStore content.Store, targetRef reference.Named, target ocispec.Descriptor) {
	appendSource, err := docker.AppendDistributionSourceLabel(realStore, targetRef.String())
	if err != nil {
		// This shouldn't happen at this point because the reference would have to be invalid
		// and if it was, then it would error out earlier.
		log.G(ctx).WithError(err).Warn("failed to create an handler that appends distribution source label to pushed content")
		return
	}

	if err := containerdimages.Dispatch(ctx, appendSource, nil, target); err != nil {
		// Shouldn't happen, but even if it would fail, then make it only a warning
		// because it doesn't affect the pushed data.
		log.G(ctx).WithError(err).Warn("failed to append distribution source labels to pushed content")
	}
}

// findMissingMountable will walk the target descriptor recursively and return
// missing contents with their distribution source which could potentially
// be cross-repo mounted.
func findMissingMountable(ctx context.Context, store content.Store, queue *jobs,
	target ocispec.Descriptor, targetRef reference.Named, limiter *semaphore.Weighted,
) (map[digest.Digest]distributionSource, error) {
	mountableBlobs := map[digest.Digest]distributionSource{}
	var mutex sync.Mutex

	sources, err := getDigestSources(ctx, store, target.Digest)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		log.G(ctx).WithField("target", target).Debug("distribution source label not found")
		return mountableBlobs, nil
	}

	handler := func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		_, err := store.Info(ctx, desc.Digest)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				return nil, errdefs.System(errors.Wrapf(err, "failed to get metadata of content %s", desc.Digest.String()))
			}

			for _, source := range sources {
				if canBeMounted(desc.MediaType, targetRef, source) {
					mutex.Lock()
					mountableBlobs[desc.Digest] = source
					mutex.Unlock()
					queue.Add(desc)
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
		return nil, errdefs.NotFound(fmt.Errorf("label %q is not attached to %s", containerdlabels.LabelDistributionSource, digest.String()))
	}

	return sources, nil
}

func extractDistributionSources(labels map[string]string) []distributionSource {
	var sources []distributionSource

	// Check if this blob has a distributionSource label
	// if yes, read it as source
	for k, v := range labels {
		if reg := strings.TrimPrefix(k, containerdlabels.LabelDistributionSource); reg != k {
			for _, repo := range strings.Split(v, ",") {
				ref, err := reference.ParseNamed(reg + "/" + repo)
				if err != nil {
					continue
				}

				sources = append(sources, distributionSource{
					registryRef: ref,
				})
			}
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
	return containerdlabels.LabelDistributionSource + domain, v
}

func (source distributionSource) GetReference(dgst digest.Digest) (reference.Named, error) {
	return reference.WithDigest(source.registryRef, dgst)
}

// canBeMounted returns if the content with given media type can be cross-repo
// mounted when pushing it to a remote reference ref.
func canBeMounted(mediaType string, targetRef reference.Named, source distributionSource) bool {
	if containerdimages.IsManifestType(mediaType) {
		return false
	}
	if containerdimages.IsIndexType(mediaType) {
		return false
	}

	reg := reference.Domain(targetRef)
	// Remove :port suffix from domain
	// containerd distribution source label doesn't store port
	if portIdx := strings.LastIndex(reg, ":"); portIdx != -1 {
		reg = reg[:portIdx]
	}

	// If the source registry is the same as the one we are pushing to
	// then the cross-repo mount will work.
	return reg == reference.Domain(source.registryRef)
}
