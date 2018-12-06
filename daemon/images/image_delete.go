package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type conflictType int

const (
	conflictDependentChild conflictType = 1 << iota
	conflictRunningContainer
	conflictActiveReference
	conflictStoppedContainer
	conflictHard = conflictDependentChild | conflictRunningContainer
	conflictSoft = conflictActiveReference | conflictStoppedContainer
)

// ImageDelete deletes the image referenced by the given imageRef from this
// daemon. The given imageRef can be an image ID, ID prefix, or a repository
// reference (with an optional tag or digest, defaulting to the tag name
// "latest"). There is differing behavior depending on whether the given
// imageRef is a repository reference or not.
//
// If the given imageRef is a repository reference then that repository
// reference will be removed. However, if there exists any containers which
// were created using the same image reference then the repository reference
// cannot be removed unless either there are other repository references to the
// same image or force is true. Following removal of the repository reference,
// the referenced image itself will attempt to be deleted as described below
// but quietly, meaning any image delete conflicts will cause the image to not
// be deleted and the conflict will not be reported.
//
// There may be conflicts preventing deletion of an image and these conflicts
// are divided into two categories grouped by their severity:
//
// Hard Conflict:
// 	- a pull or build using the image.
// 	- any descendant image.
// 	- any running container using the image.
//
// Soft Conflict:
// 	- any stopped container using the image.
// 	- any repository tag or digest references to the image.
//
// The image cannot be removed if there are any hard conflicts and can be
// removed if there are soft conflicts only if force is true.
//
// If prune is true, ancestor images will each attempt to be deleted quietly,
// meaning any delete conflicts will cause the image to not be deleted and the
// conflict will not be reported.
//
func (i *ImageService) ImageDelete(ctx context.Context, imageRef string, force, prune bool) ([]types.ImageDeleteResponseItem, error) {
	start := time.Now()

	img, err := i.getCachedRef(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	imgID := img.config.Digest
	repoRefs := img.references

	using := func(c *container.Container) bool {
		return digest.Digest(c.ImageID) == imgID
	}

	var deletedRefs []reference.Named
	var removedRepositoryRef bool
	if !isImageIDPrefix(imgID.String(), imageRef) {
		// A repository reference was given and should be removed
		// first. We can only remove this reference if either force is
		// true, there are multiple repository references to this
		// image, or there are no containers using the given reference.
		if !force && isSingleReference(repoRefs) {
			if container := i.containers.First(using); container != nil {
				// If we removed the repository reference then
				// this image would remain "dangling" and since
				// we really want to avoid that the client must
				// explicitly force its removal.
				err := errors.Errorf("conflict: unable to remove repository reference %q (must force) - container %s is using its referenced image %s", imageRef, stringid.TruncateID(container.ID), stringid.TruncateID(imgID.String()))
				return nil, errdefs.Conflict(err)
			}
		}

		parsedRef, err := reference.ParseNormalizedNamed(imageRef)
		if err != nil {
			return nil, err
		}

		deletedRefs = append(deletedRefs, parsedRef)
		i.LogImageEvent(imgID.String(), imgID.String(), "untag")

		// If a tag reference was removed and the only remaining
		// references to the same repository are digest references,
		// then clean up those digest references.
		if _, isCanonical := parsedRef.(reference.Canonical); !isCanonical {
			foundRepoTagRef := false
			for _, repoRef := range repoRefs {
				if parsedRef.String() == repoRef.String() {
					continue
				}
				if _, repoRefIsCanonical := repoRef.(reference.Canonical); !repoRefIsCanonical && parsedRef.Name() == repoRef.Name() {
					foundRepoTagRef = true
					break
				}
			}
			if !foundRepoTagRef {
				// Remove canonical references from same repository
				for _, repoRef := range repoRefs {
					if parsedRef.String() == repoRef.String() {
						continue
					}
					if _, repoRefIsCanonical := repoRef.(reference.Canonical); repoRefIsCanonical && parsedRef.Name() == repoRef.Name() {
						// TODO(containerd): can repoRef be name only here?
						deletedRefs = append(deletedRefs, repoRef)
					}
				}
			}
		}

		// If it has remaining references then the untag finished the remove
		if len(repoRefs)-len(deletedRefs) > 0 {
			// Remove all references in containerd
			// Do not wait for containerd's garbage collection
			records, err := i.removeImageRefs(ctx, deletedRefs, false)
			if err != nil {
				return nil, errors.Wrap(err, "failed to delete refs")
			}
			return records, nil
		}

		removedRepositoryRef = true
	} else {
		// If an ID reference was given AND there is at most one tag
		// reference to the image AND all references are within one
		// repository, then remove all references.
		if isSingleReference(repoRefs) {
			c := conflictHard
			if !force {
				c |= conflictSoft &^ conflictActiveReference
			}
			if conflict := i.checkImageDeleteConflict(imgID, c); conflict != nil {
				return nil, conflict
			}

			for _, repoRef := range repoRefs {
				// TODO(containerd): can repoRef be name only here?
				deletedRefs = append(deletedRefs, repoRef)
				i.LogImageEvent(imgID.String(), imgID.String(), "untag")
			}
		}
	}

	// TODO(containerd): Lock, perform deletion,
	// check if image exists then delete layers
	records, err := i.imageDeleteHelper(ctx, img, repoRefs, force, prune, removedRepositoryRef)
	if err != nil {
		return nil, err
	}

	imageActions.WithValues("delete").UpdateSince(start)

	return records, nil
}

// isSingleReference returns true when all references are from one repository
// and there is at most one tag. Returns false for empty input.
func isSingleReference(repoRefs []reference.Named) bool {
	if len(repoRefs) <= 1 {
		return len(repoRefs) == 1
	}
	var singleRef reference.Named
	canonicalRefs := map[string]struct{}{}
	for _, repoRef := range repoRefs {
		if _, isCanonical := repoRef.(reference.Canonical); isCanonical {
			canonicalRefs[repoRef.Name()] = struct{}{}
		} else if singleRef == nil {
			singleRef = repoRef
		} else {
			return false
		}
	}
	if singleRef == nil {
		// Just use first canonical ref
		singleRef = repoRefs[0]
	}
	_, ok := canonicalRefs[singleRef.Name()]
	return len(canonicalRefs) == 1 && ok
}

// isImageIDPrefix returns whether the given possiblePrefix is a prefix of the
// given imageID.
func isImageIDPrefix(imageID, possiblePrefix string) bool {
	if strings.HasPrefix(imageID, possiblePrefix) {
		return true
	}

	if i := strings.IndexRune(imageID, ':'); i >= 0 {
		return strings.HasPrefix(imageID[i+1:], possiblePrefix)
	}

	return false
}

// removeImageRefs removes a set of image references
// if the sync flag is set then garbage collection is
// is completed before returning
func (i *ImageService) removeImageRefs(ctx context.Context, refs []reference.Named, sync bool) ([]types.ImageDeleteResponseItem, error) {
	records := []types.ImageDeleteResponseItem{}
	// TODO(containerd): clear from cache, get cache from arguments

	is := i.client.ImageService()

	for i, ref := range refs {
		opts := []images.DeleteOpt{}
		if sync && i == len(refs)-1 {
			opts = append(opts, images.SynchronousDelete())
		}
		if err := is.Delete(ctx, ref.String(), opts...); err != nil && !errdefs.IsNotFound(err) {
			return records, errors.Wrapf(err, "failed to delete ref: %s", ref.String())
		}

		// TODO(containerd): do this?
		//i.LogImageEvent(imgID.String(), imgID.String(), "untag")

		untaggedRecord := types.ImageDeleteResponseItem{Untagged: reference.FamiliarString(ref)}
		records = append(records, untaggedRecord)
	}

	return records, nil
}

// ImageDeleteConflict holds a soft or hard conflict and an associated error.
// Implements the error interface.
type imageDeleteConflict struct {
	hard    bool
	used    bool
	imgID   image.ID
	message string
}

func (idc *imageDeleteConflict) Error() string {
	var forceMsg string
	if idc.hard {
		forceMsg = "cannot be forced"
	} else {
		forceMsg = "must be forced"
	}

	return fmt.Sprintf("conflict: unable to delete %s (%s) - %s", stringid.TruncateID(idc.imgID.String()), forceMsg, idc.message)
}

func (idc *imageDeleteConflict) Conflict() {}

// imageDeleteHelper attempts to delete the given image from this daemon. If
// the image has any hard delete conflicts (child images or running containers
// using the image) then it cannot be deleted. If the image has any soft delete
// conflicts (any tags/digests referencing the image or any stopped container
// using the image) then it can only be deleted if force is true. If the delete
// succeeds and prune is true, the parent images are also deleted if they do
// not have any soft or hard delete conflicts themselves. Any deleted images
// and untagged references are appended to the given records. If any error or
// conflict is encountered, it will be returned immediately without deleting
// the image. If quiet is true, any encountered conflicts will be ignored and
// the function will return nil immediately without deleting the image.
func (i *ImageService) imageDeleteHelper(ctx context.Context, img *cachedImage, repoRefs []reference.Named, force, prune, quiet bool) ([]types.ImageDeleteResponseItem, error) {
	// TODO(containerd): lock deletion, make reference removal and checks transactional in the cache?

	// First, determine if this image has any conflicts. Ignore soft conflicts
	// if force is true.
	c := conflictHard
	if !force {
		c |= conflictSoft
	}
	if conflict := i.checkImageDeleteConflict(img.config.Digest, c); conflict != nil {
		if quiet && (!i.imageIsDangling(img.config.Digest) || conflict.used) {
			// Ignore conflicts UNLESS the image is "dangling" or not being used in
			// which case we want the user to know.
			return nil, nil
		}

		// There was a conflict and it's either a hard conflict OR we are not
		// forcing deletion on soft conflicts.
		return nil, conflict
	}

	// Delete all repository tag/digest references to this image.
	records, err := i.removeImageRefs(ctx, repoRefs, true)
	if err != nil {
		return records, err
	}

	// NOTE(containerd): GC can do this in the future
	// TODO(containerd): Move this function locally, to track and release layers
	// Walk layers and remove reference
	removedLayers, err := i.imageStore.Delete(image.ID(img.config.Digest))
	if err != nil {
		return records, err
	}

	i.LogImageEvent(img.config.Digest.String(), img.config.Digest.String(), "delete")
	records = append(records, types.ImageDeleteResponseItem{Deleted: img.config.Digest.String()})
	for _, removedLayer := range removedLayers {
		records = append(records, types.ImageDeleteResponseItem{Deleted: removedLayer.ChainID.String()})
	}

	var parent *cachedImage
	if img.parent != "" {
		// TODO(containerd): pass cache in
		c, err := i.getCache(ctx)
		if err != nil {
			return records, err
		}
		parent = c.byID(img.parent)
	}

	if !prune || parent == nil {
		return records, nil
	}

	// We need to prune the parent image. This means delete it if there are
	// no tags/digests referencing it and there are no containers using it (
	// either running or stopped).
	// Do not force prunings, but do so quietly (stopping on any encountered
	// conflicts).
	parentRecords, err := i.imageDeleteHelper(ctx, parent, nil, false, true, true)
	return append(records, parentRecords...), nil
}

// checkImageDeleteConflict determines whether there are any conflicts
// preventing deletion of the given image from this daemon. A hard conflict is
// any image which has the given image as a parent or any running container
// using the image. A soft conflict is any tags/digest referencing the given
// image or any stopped container using the image. If ignoreSoftConflicts is
// true, this function will not check for soft conflict conditions.
func (i *ImageService) checkImageDeleteConflict(imgID digest.Digest, mask conflictType) *imageDeleteConflict {
	// Check if the image has any descendant images.
	// TODO(containerd): No use of image store
	if mask&conflictDependentChild != 0 && len(i.imageStore.Children(image.ID(imgID))) > 0 {
		return &imageDeleteConflict{
			hard:    true,
			imgID:   image.ID(imgID),
			message: "image has dependent child images",
		}
	}

	if mask&conflictRunningContainer != 0 {
		// Check if any running container is using the image.
		running := func(c *container.Container) bool {
			return c.IsRunning() && digest.Digest(c.ImageID) == imgID
		}
		if container := i.containers.First(running); container != nil {
			return &imageDeleteConflict{
				imgID:   image.ID(imgID),
				hard:    true,
				used:    true,
				message: fmt.Sprintf("image is being used by running container %s", stringid.TruncateID(container.ID)),
			}
		}
	}

	// Check if any repository tags/digest reference this image.
	// TODO(containerd): No use of reference store
	if mask&conflictActiveReference != 0 && len(i.referenceStore.References(imgID)) > 0 {
		return &imageDeleteConflict{
			imgID:   image.ID(imgID),
			message: "image is referenced in multiple repositories",
		}
	}

	if mask&conflictStoppedContainer != 0 {
		// Check if any stopped containers reference this image.
		stopped := func(c *container.Container) bool {
			return !c.IsRunning() && digest.Digest(c.ImageID) == imgID
		}
		if container := i.containers.First(stopped); container != nil {
			return &imageDeleteConflict{
				imgID:   image.ID(imgID),
				used:    true,
				message: fmt.Sprintf("image is being used by stopped container %s", stringid.TruncateID(container.ID)),
			}
		}
	}

	return nil
}

// imageIsDangling returns whether the given image is "dangling" which means
// that there are no repository references to the given image and it has no
// child images.
func (i *ImageService) imageIsDangling(imgID digest.Digest) bool {
	// TODO(containerd): No use of reference store
	// TODO(containerd): No use of image store
	// To find children, Docker keeps a cache of images along with parents, it
	// can also keep a backpointer to parents in memory
	return !(len(i.referenceStore.References(imgID)) > 0 || len(i.imageStore.Children(image.ID(imgID))) > 0)
}
