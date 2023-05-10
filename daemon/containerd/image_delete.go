package containerd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
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
//   - any running container using the image.
//
// Soft Conflict:
//   - any stopped container using the image.
//   - any repository tag or digest references to the image.
//
// The image cannot be removed if there are any hard conflicts and can be
// removed if there are soft conflicts only if force is true.
//
// If prune is true, ancestor images will each attempt to be deleted quietly,
// meaning any delete conflicts will cause the image to not be deleted and the
// conflict will not be reported.
//
// TODO(thaJeztah): image delete should send prometheus counters; see https://github.com/moby/moby/issues/45268
func (i *ImageService) ImageDelete(ctx context.Context, imageRef string, force, prune bool) ([]types.ImageDeleteResponseItem, error) {
	parsedRef, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return nil, err
	}

	img, err := i.resolveImage(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	imgID := image.ID(img.Target.Digest)

	if isImageIDPrefix(imgID.String(), imageRef) {
		return i.deleteAll(ctx, img, force, prune)
	}

	singleRef, err := i.isSingleReference(ctx, img)
	if err != nil {
		return nil, err
	}
	if !singleRef {
		err := i.client.ImageService().Delete(ctx, img.Name)
		if err != nil {
			return nil, err
		}
		i.LogImageEvent(imgID.String(), imgID.String(), "untag")
		records := []types.ImageDeleteResponseItem{{Untagged: reference.FamiliarString(reference.TagNameOnly(parsedRef))}}
		return records, nil
	}

	using := func(c *container.Container) bool {
		return c.ImageID == imgID
	}
	ctr := i.containers.First(using)
	if ctr != nil {
		if !force {
			// If we removed the repository reference then
			// this image would remain "dangling" and since
			// we really want to avoid that the client must
			// explicitly force its removal.
			refString := reference.FamiliarString(reference.TagNameOnly(parsedRef))
			err := &imageDeleteConflict{
				reference: refString,
				used:      true,
				message: fmt.Sprintf("container %s is using its referenced image %s",
					stringid.TruncateID(ctr.ID),
					stringid.TruncateID(imgID.String())),
			}
			return nil, err
		}

		err := i.softImageDelete(ctx, img)
		if err != nil {
			return nil, err
		}

		i.LogImageEvent(imgID.String(), imgID.String(), "untag")
		records := []types.ImageDeleteResponseItem{{Untagged: reference.FamiliarString(reference.TagNameOnly(parsedRef))}}
		return records, nil
	}

	return i.deleteAll(ctx, img, force, prune)
}

// deleteAll deletes the image from the daemon, and if prune is true,
// also deletes dangling parents if there is no conflict in doing so.
// Parent images are removed quietly, and if there is any issue/conflict
// it is logged but does not halt execution/an error is not returned.
func (i *ImageService) deleteAll(ctx context.Context, img images.Image, force, prune bool) ([]types.ImageDeleteResponseItem, error) {
	var records []types.ImageDeleteResponseItem

	// Workaround for: https://github.com/moby/buildkit/issues/3797
	possiblyDeletedConfigs := map[digest.Digest]struct{}{}
	err := i.walkPresentChildren(ctx, img.Target, func(_ context.Context, d ocispec.Descriptor) {
		if images.IsConfigType(d.MediaType) {
			possiblyDeletedConfigs[d.Digest] = struct{}{}
		}
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := i.unleaseSnapshotsFromDeletedConfigs(context.Background(), possiblyDeletedConfigs); err != nil {
			logrus.WithError(err).Warn("failed to unlease snapshots")
		}
	}()

	imgID := img.Target.Digest.String()

	var parents []imageWithRootfs
	if prune {
		parents, err = i.parents(ctx, image.ID(imgID))
		if err != nil {
			logrus.WithError(err).Warn("failed to get image parents")
		}
		sortParentsByAffinity(parents)
	}

	imageRefs, err := i.client.ImageService().List(ctx, "target.digest=="+imgID)
	if err != nil {
		return nil, err
	}
	for _, imageRef := range imageRefs {
		if err := i.imageDeleteHelper(ctx, imageRef, &records, force); err != nil {
			return records, err
		}
	}
	i.LogImageEvent(imgID, imgID, "delete")
	records = append(records, types.ImageDeleteResponseItem{Deleted: imgID})

	for _, parent := range parents {
		if !isDanglingImage(parent.img) {
			break
		}
		err = i.imageDeleteHelper(ctx, parent.img, &records, false)
		if err != nil {
			logrus.WithError(err).Warn("failed to remove image parent")
			break
		}
		parentID := parent.img.Target.Digest.String()
		i.LogImageEvent(parentID, parentID, "delete")
		records = append(records, types.ImageDeleteResponseItem{Deleted: parentID})
	}

	return records, nil
}

// isImageIDPrefix returns whether the given
// possiblePrefix is a prefix of the given imageID.
func isImageIDPrefix(imageID, possiblePrefix string) bool {
	if strings.HasPrefix(imageID, possiblePrefix) {
		return true
	}
	if i := strings.IndexRune(imageID, ':'); i >= 0 {
		return strings.HasPrefix(imageID[i+1:], possiblePrefix)
	}
	return false
}

func sortParentsByAffinity(parents []imageWithRootfs) {
	sort.Slice(parents, func(i, j int) bool {
		lenRootfsI := len(parents[i].rootfs.DiffIDs)
		lenRootfsJ := len(parents[j].rootfs.DiffIDs)
		if lenRootfsI == lenRootfsJ {
			return isDanglingImage(parents[i].img)
		}
		return lenRootfsI > lenRootfsJ
	})
}

// isSingleReference returns true if there are no other images in the
// daemon targeting the same content as `img` that are not dangling.
func (i *ImageService) isSingleReference(ctx context.Context, img images.Image) (bool, error) {
	refs, err := i.client.ImageService().List(ctx, "target.digest=="+img.Target.Digest.String())
	if err != nil {
		return false, err
	}
	for _, ref := range refs {
		if !isDanglingImage(ref) && ref.Name != img.Name {
			return false, nil
		}
	}
	return true, nil
}

type conflictType int

const (
	conflictRunningContainer conflictType = 1 << iota
	conflictActiveReference
	conflictStoppedContainer
	conflictHard = conflictRunningContainer
	conflictSoft = conflictActiveReference | conflictStoppedContainer
)

// imageDeleteHelper attempts to delete the given image from this daemon.
// If the image has any hard delete conflicts (running containers using
// the image) then it cannot be deleted. If the image has any soft delete
// conflicts (any tags/digests referencing the image or any stopped container
// using the image) then it can only be deleted if force is true. Any deleted
// images and untagged references are appended to the given records. If any
// error or conflict is encountered, it will be returned immediately without
// deleting the image.
func (i *ImageService) imageDeleteHelper(ctx context.Context, img images.Image, records *[]types.ImageDeleteResponseItem, force bool) error {
	// First, determine if this image has any conflicts. Ignore soft conflicts
	// if force is true.
	c := conflictHard
	if !force {
		c |= conflictSoft
	}

	imgID := image.ID(img.Target.Digest)

	err := i.checkImageDeleteConflict(ctx, imgID, c)
	if err != nil {
		return err
	}

	untaggedRef, err := reference.ParseAnyReference(img.Name)
	if err != nil {
		return err
	}
	err = i.client.ImageService().Delete(ctx, img.Name, images.SynchronousDelete())
	if err != nil {
		return err
	}

	i.LogImageEvent(imgID.String(), imgID.String(), "untag")
	*records = append(*records, types.ImageDeleteResponseItem{Untagged: reference.FamiliarString(untaggedRef)})

	return nil
}

// ImageDeleteConflict holds a soft or hard conflict and associated
// error. A hard conflict represents a running container using the
// image, while a soft conflict is any tags/digests referencing the
// given image or any stopped container using the image.
// Implements the error interface.
type imageDeleteConflict struct {
	hard      bool
	used      bool
	reference string
	message   string
}

func (idc *imageDeleteConflict) Error() string {
	var forceMsg string
	if idc.hard {
		forceMsg = "cannot be forced"
	} else {
		forceMsg = "must be forced"
	}
	return fmt.Sprintf("conflict: unable to delete %s (%s) - %s", idc.reference, forceMsg, idc.message)
}

func (imageDeleteConflict) Conflict() {}

// checkImageDeleteConflict returns a conflict representing
// any issue preventing deletion of the given image ID, and
// nil if there are none. It takes a bitmask representing a
// filter for which conflict types the caller cares about,
// and will only check for these conflict types.
func (i *ImageService) checkImageDeleteConflict(ctx context.Context, imgID image.ID, mask conflictType) error {
	if mask&conflictRunningContainer != 0 {
		running := func(c *container.Container) bool {
			return c.ImageID == imgID && c.IsRunning()
		}
		if ctr := i.containers.First(running); ctr != nil {
			return &imageDeleteConflict{
				reference: stringid.TruncateID(imgID.String()),
				hard:      true,
				used:      true,
				message:   fmt.Sprintf("image is being used by running container %s", stringid.TruncateID(ctr.ID)),
			}
		}
	}

	if mask&conflictStoppedContainer != 0 {
		stopped := func(c *container.Container) bool {
			return !c.IsRunning() && c.ImageID == imgID
		}
		if ctr := i.containers.First(stopped); ctr != nil {
			return &imageDeleteConflict{
				reference: stringid.TruncateID(imgID.String()),
				used:      true,
				message:   fmt.Sprintf("image is being used by stopped container %s", stringid.TruncateID(ctr.ID)),
			}
		}
	}

	if mask&conflictActiveReference != 0 {
		refs, err := i.client.ImageService().List(ctx, "target.digest=="+imgID.String())
		if err != nil {
			return err
		}
		if len(refs) > 1 {
			return &imageDeleteConflict{
				reference: stringid.TruncateID(imgID.String()),
				message:   "image is referenced in multiple repositories",
			}
		}
	}

	return nil
}
