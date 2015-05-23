package daemon

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/docker/utils"
)

// FIXME: remove ImageDelete's dependency on Daemon, then move to graph/
func (daemon *Daemon) ImageDelete(name string, force, noprune bool) ([]types.ImageDelete, error) {
	list := []types.ImageDelete{}
	if err := daemon.imgDeleteHelper(name, &list, true, force, noprune); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("Conflict, %s wasn't deleted", name)
	}

	return list, nil
}

var errTagRemovable = fmt.Errorf("tag removable")

func (daemon *Daemon) imgDeleteHelper(name string, list *[]types.ImageDelete, first, force, noprune bool) error {
	var repoName, tag string
	repoAndTags := make(map[string][]string)

	// FIXME: please respect DRY and centralize repo+tag parsing in a single central place! -- shykes
	repoName, tag = parsers.ParseRepositoryTag(name)
	if tag == "" {
		tag = tags.DefaultTag
	}

	if name == "" {
		return fmt.Errorf("Image name can not be blank")
	}

	img, err := daemon.Repositories().LookupImage(name)
	if err != nil {
		if r, _ := daemon.Repositories().Get(repoName); r != nil {
			return fmt.Errorf("No such image: %s", utils.ImageReference(repoName, tag))
		}
		return fmt.Errorf("No such image: %s", name)
	}

	if strings.Contains(img.ID, name) {
		repoName = ""
		tag = ""
	}

	byParents := daemon.Graph().ByParent()

	repos := daemon.Repositories().ByID()[img.ID]

	//If delete by id, see if the id belong only to one repository
	deleteByID := repoName == ""
	if deleteByID {
		for _, repoAndTag := range repos {
			parsedRepo, parsedTag := parsers.ParseRepositoryTag(repoAndTag)
			if repoName == "" || repoName == parsedRepo {
				repoName = parsedRepo
				if parsedTag != "" {
					repoAndTags[repoName] = append(repoAndTags[repoName], parsedTag)
				}
			} else if repoName != parsedRepo && !force && first {
				// the id belongs to multiple repos, like base:latest and user:test,
				// in that case return conflict
				return fmt.Errorf("Conflict, cannot delete image %s because it is tagged in multiple repositories, use -f to force", name)
			} else {
				//the id belongs to multiple repos, with -f just delete all
				repoName = parsedRepo
				if parsedTag != "" {
					repoAndTags[repoName] = append(repoAndTags[repoName], parsedTag)
				}
			}
		}
	} else {
		repoAndTags[repoName] = append(repoAndTags[repoName], tag)
	}

	if !first && len(repoAndTags) > 0 {
		return nil
	}

	// Untag the current image
	untag := func(repoAndTags map[string][]string) error {
		for repoName, tags := range repoAndTags {
			for _, tag := range tags {
				tagDeleted, err := daemon.Repositories().Delete(repoName, tag)
				if err != nil {
					return err
				}
				if !tagDeleted {
					continue
				}

				*list = append(*list, types.ImageDelete{
					Untagged: utils.ImageReference(repoName, tag),
				})
				daemon.EventsService.Log("untag", img.ID, "")
			}
		}
		return nil
	}

	if len(repos) <= 1 || deleteByID {
		if err := daemon.canDeleteImage(img.ID, force); err != nil {
			if err == errTagRemovable && !deleteByID {
				return untag(repoAndTags)
			} else {
				return err
			}
		}
	}

	if err := untag(repoAndTags); err != nil {
		return err
	}
	tags := daemon.Repositories().ByID()[img.ID]
	if (len(tags) <= 1 && repoName == "") || len(tags) == 0 {
		if len(byParents[img.ID]) == 0 {
			if err := daemon.Repositories().DeleteAll(img.ID); err != nil {
				return err
			}
			if err := daemon.Graph().Delete(img.ID); err != nil {
				return err
			}
			*list = append(*list, types.ImageDelete{
				Deleted: img.ID,
			})
			daemon.EventsService.Log("delete", img.ID, "")
			if img.Parent != "" && !noprune {
				err := daemon.imgDeleteHelper(img.Parent, list, false, force, noprune)
				if first {
					return err
				}
			}
		}
	}
	return nil
}

func (daemon *Daemon) canDeleteImage(imgID string, force bool) error {
	if daemon.Graph().IsHeld(imgID) {
		return fmt.Errorf("Conflict, cannot delete because %s is held by an ongoing pull or build", stringid.TruncateID(imgID))
	}
	directlyUsedBy := []string{}
	indirectlyUsedBy := []string{}

	for _, container := range daemon.List() {
		if container.ImageID == "" {
			// This technically should never happen, but if the container
			// has no ImageID then log the situation and move on.
			// If we allowed processing to continue then the code later
			// on would fail with a "Prefix can't be empty" error even
			// though the bad container has nothing to do with the image
			// we're trying to delete.
			logrus.Errorf("Container %q has no image associated with it!", container.ID)
			continue
		}
		parent, err := daemon.Repositories().LookupImage(container.ImageID)
		if err != nil {
			if daemon.Graph().IsNotExist(err, container.ImageID) {
				continue
			}
			return err
		}

		direct := true
		if err := daemon.graph.WalkHistory(parent, func(p image.Image) error {
			if imgID == p.ID {
				if direct {
					directlyUsedBy = append(directlyUsedBy, container.ID)
				} else {
					indirectlyUsedBy = append(indirectlyUsedBy, container.ID)
				}
			}
			direct = false
			return nil
		}); err != nil {
			return err
		}
	}

	// The image can be deleted because no containers are using the image.
	if len(directlyUsedBy) == 0 && len(indirectlyUsedBy) == 0 {
		return nil
	}

	// layer2 - with container "bar" using it
	// layer1 - with the tag "foo" but no containers are using the tag
	// layer0
	// In this case "foo" can be untagged safely.
	if len(directlyUsedBy) == 0 {
		return errTagRemovable
	}

	// The image is a parent layer of another image.
	// At the same, there are some containers using it.
	// If the -f flag is provided we can try to untag it.
	if len(indirectlyUsedBy) != 0 {
		if force {
			return errTagRemovable
		}
		return fmt.Errorf("Conflict, cannot delete %s because the container %s is using it. But you can use -f to untag it", stringid.TruncateID(imgID), stringid.TruncateID(directlyUsedBy[0]))
	}

	// The image is directly used by some containers.
	// It depends on the state of the containers and the -f flag.
	for _, cid := range directlyUsedBy {
		container, err := daemon.Get(cid)
		if err != nil {
			if truncindex.IsNotFound(err) {
				// The container might had been deleted after the previous call to daemon.List.
				continue
			}
			return err
		}

		if !container.IsRunning() {
			if !force {
				return fmt.Errorf("Conflict, cannot delete %s because the container %s is using it, use -f to force", stringid.TruncateID(imgID), stringid.TruncateID(cid))
			}
			continue
		}

		if force {
			return fmt.Errorf("Conflict, cannot force delete %s because the running container %s is using it, stop it and retry", stringid.TruncateID(imgID), stringid.TruncateID(cid))
		}

		return fmt.Errorf("Conflict, cannot delete %s because the running container %s is using it, stop it and use -f to force", stringid.TruncateID(imgID), stringid.TruncateID(cid))
	}
	return nil
}
