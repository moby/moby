package daemon

import (
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	tagpkg "github.com/docker/docker/tag"
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
// 	- any descendent image.
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
// FIXME: remove ImageDelete's dependency on Daemon, then move to the graph
// package. This would require that we no longer need the daemon to determine
// whether images are being used by a stopped or running container.
func (daemon *Daemon) ImageDelete(imageRef string, force, prune bool) ([]types.ImageDelete, error) {
	records := []types.ImageDelete{}

	imgID, err := daemon.GetImageID(imageRef)
	if err != nil {
		return nil, daemon.imageNotExistToErrcode(err)
	}

	repoRefs := daemon.tagStore.References(imgID)

	var removedRepositoryRef bool
	if !isImageIDPrefix(imgID.String(), imageRef) {
		// A repository reference was given and should be removed
		// first. We can only remove this reference if either force is
		// true, there are multiple repository references to this
		// image, or there are no containers using the given reference.
		if !(force || len(repoRefs) > 1) {
			if container := daemon.getContainerUsingImage(imgID); container != nil {
				// If we removed the repository reference then
				// this image would remain "dangling" and since
				// we really want to avoid that the client must
				// explicitly force its removal.
				return nil, derr.ErrorCodeImgDelUsed.WithArgs(imageRef, stringid.TruncateID(container.ID), stringid.TruncateID(imgID.String()))
			}
		}

		parsedRef, err := reference.ParseNamed(imageRef)
		if err != nil {
			return nil, err
		}

		parsedRef, err = daemon.removeImageRef(parsedRef)
		if err != nil {
			return nil, err
		}

		untaggedRecord := types.ImageDelete{Untagged: parsedRef.String()}

		daemon.EventsService.Log("untag", imgID.String(), "")
		records = append(records, untaggedRecord)

		// If has remaining references then untag finishes the remove
		if len(repoRefs) > 1 {
			return records, nil
		}

		removedRepositoryRef = true
	} else {
		// If an ID reference was given AND there is exactly one
		// repository reference to the image then we will want to
		// remove that reference.
		// FIXME: Is this the behavior we want?
		if len(repoRefs) == 1 {
			parsedRef, err := daemon.removeImageRef(repoRefs[0])
			if err != nil {
				return nil, err
			}

			untaggedRecord := types.ImageDelete{Untagged: parsedRef.String()}

			daemon.EventsService.Log("untag", imgID.String(), "")
			records = append(records, untaggedRecord)
		}
	}

	return records, daemon.imageDeleteHelper(imgID, &records, force, prune, removedRepositoryRef)
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

// getContainerUsingImage returns a container that was created using the given
// imageID. Returns nil if there is no such container.
func (daemon *Daemon) getContainerUsingImage(imageID image.ID) *Container {
	for _, container := range daemon.List() {
		if container.ImageID == imageID {
			return container
		}
	}

	return nil
}

// removeImageRef attempts to parse and remove the given image reference from
// this daemon's store of repository tag/digest references. The given
// repositoryRef must not be an image ID but a repository name followed by an
// optional tag or digest reference. If tag or digest is omitted, the default
// tag is used. Returns the resolved image reference and an error.
func (daemon *Daemon) removeImageRef(ref reference.Named) (reference.Named, error) {
	switch ref.(type) {
	case reference.Tagged:
	case reference.Digested:
	default:
		var err error
		ref, err = reference.WithTag(ref, tagpkg.DefaultTag)
		if err != nil {
			return nil, err
		}
	}

	// Ignore the boolean value returned, as far as we're concerned, this
	// is an idempotent operation and it's okay if the reference didn't
	// exist in the first place.
	_, err := daemon.tagStore.Delete(ref)

	return ref, err
}

// removeAllReferencesToImageID attempts to remove every reference to the given
// imgID from this daemon's store of repository tag/digest references. Returns
// on the first encountered error. Removed references are logged to this
// daemon's event service. An "Untagged" types.ImageDelete is added to the
// given list of records.
func (daemon *Daemon) removeAllReferencesToImageID(imgID image.ID, records *[]types.ImageDelete) error {
	imageRefs := daemon.tagStore.References(imgID)

	for _, imageRef := range imageRefs {
		parsedRef, err := daemon.removeImageRef(imageRef)
		if err != nil {
			return err
		}

		untaggedRecord := types.ImageDelete{Untagged: parsedRef.String()}

		daemon.EventsService.Log("untag", imgID.String(), "")
		*records = append(*records, untaggedRecord)
	}

	return nil
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
func (daemon *Daemon) imageDeleteHelper(imgID image.ID, records *[]types.ImageDelete, force, prune, quiet bool) error {
	// First, determine if this image has any conflicts. Ignore soft conflicts
	// if force is true.
	if conflict := daemon.checkImageDeleteConflict(imgID, force); conflict != nil {
		if quiet && (!daemon.imageIsDangling(imgID) || conflict.used) {
			// Ignore conflicts UNLESS the image is "dangling" or not being used in
			// which case we want the user to know.
			return nil
		}

		// There was a conflict and it's either a hard conflict OR we are not
		// forcing deletion on soft conflicts.
		return conflict
	}

	parent, err := daemon.imageStore.GetParent(imgID)
	if err != nil {
		// There may be no parent
		parent = ""
	}

	// Delete all repository tag/digest references to this image.
	if err := daemon.removeAllReferencesToImageID(imgID, records); err != nil {
		return err
	}

	removedLayers, err := daemon.imageStore.Delete(imgID)
	if err != nil {
		return err
	}

	daemon.EventsService.Log("delete", imgID.String(), "")
	*records = append(*records, types.ImageDelete{Deleted: imgID.String()})
	for _, removedLayer := range removedLayers {
		*records = append(*records, types.ImageDelete{Deleted: removedLayer.ChainID.String()})
	}

	if !prune || parent == "" {
		return nil
	}

	// We need to prune the parent image. This means delete it if there are
	// no tags/digests referencing it and there are no containers using it (
	// either running or stopped).
	// Do not force prunings, but do so quietly (stopping on any encountered
	// conflicts).
	return daemon.imageDeleteHelper(parent, records, false, true, true)
}

// checkImageDeleteConflict determines whether there are any conflicts
// preventing deletion of the given image from this daemon. A hard conflict is
// any image which has the given image as a parent or any running container
// using the image. A soft conflict is any tags/digest referencing the given
// image or any stopped container using the image. If ignoreSoftConflicts is
// true, this function will not check for soft conflict conditions.
func (daemon *Daemon) checkImageDeleteConflict(imgID image.ID, ignoreSoftConflicts bool) *imageDeleteConflict {
	// Check for hard conflicts first.
	if conflict := daemon.checkImageDeleteHardConflict(imgID); conflict != nil {
		return conflict
	}

	// Then check for soft conflicts.
	if ignoreSoftConflicts {
		// Don't bother checking for soft conflicts.
		return nil
	}

	return daemon.checkImageDeleteSoftConflict(imgID)
}

func (daemon *Daemon) checkImageDeleteHardConflict(imgID image.ID) *imageDeleteConflict {
	// Check if the image has any descendent images.
	if len(daemon.imageStore.Children(imgID)) > 0 {
		return &imageDeleteConflict{
			hard:    true,
			imgID:   imgID,
			message: "image has dependent child images",
		}
	}

	// Check if any running container is using the image.
	for _, container := range daemon.List() {
		if !container.IsRunning() {
			// Skip this until we check for soft conflicts later.
			continue
		}

		if container.ImageID == imgID {
			return &imageDeleteConflict{
				imgID:   imgID,
				hard:    true,
				used:    true,
				message: fmt.Sprintf("image is being used by running container %s", stringid.TruncateID(container.ID)),
			}
		}
	}

	return nil
}

func (daemon *Daemon) checkImageDeleteSoftConflict(imgID image.ID) *imageDeleteConflict {
	// Check if any repository tags/digest reference this image.
	if len(daemon.tagStore.References(imgID)) > 0 {
		return &imageDeleteConflict{
			imgID:   imgID,
			message: "image is referenced in one or more repositories",
		}
	}

	// Check if any stopped containers reference this image.
	for _, container := range daemon.List() {
		if container.IsRunning() {
			// Skip this as it was checked above in hard conflict conditions.
			continue
		}

		if container.ImageID == imgID {
			return &imageDeleteConflict{
				imgID:   imgID,
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
func (daemon *Daemon) imageIsDangling(imgID image.ID) bool {
	return !(len(daemon.tagStore.References(imgID)) > 0 || len(daemon.imageStore.Children(imgID)) > 0)
}
