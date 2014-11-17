package daemon

import (
	"fmt"
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/utils"
)

func (daemon *Daemon) ImageDelete(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s IMAGE", job.Name)
	}
	imgs := engine.NewTable("", 0)
	if err := daemon.DeleteImage(job.Eng, job.Args[0], imgs, true, job.GetenvBool("force"), job.GetenvBool("noprune")); err != nil {
		return job.Error(err)
	}
	if len(imgs.Data) == 0 {
		return job.Errorf("Conflict, %s wasn't deleted", job.Args[0])
	}
	if _, err := imgs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

// FIXME: make this private and use the job instead
func (daemon *Daemon) DeleteImage(eng *engine.Engine, name string, imgs *engine.Table, first, force, noprune bool) error {
	var (
		repoName, tag string
		tags          = []string{}
	)

	// FIXME: please respect DRY and centralize repo+tag parsing in a single central place! -- shykes
	repoName, tag = parsers.ParseRepositoryTag(name)
	if tag == "" {
		tag = graph.DEFAULTTAG
	}

	img, err := daemon.Repositories().LookupImage(name)
	if err != nil {
		if r, _ := daemon.Repositories().Get(repoName); r != nil {
			return fmt.Errorf("No such image: %s:%s", repoName, tag)
		}
		return fmt.Errorf("No such image: %s", name)
	}

	if strings.Contains(img.ID, name) {
		repoName = ""
		tag = ""
	}

	byParents, err := daemon.Graph().ByParent()
	if err != nil {
		return err
	}

	repos := daemon.Repositories().ByID()[img.ID]

	//If delete by id, see if the id belong only to one repository
	if repoName == "" {
		for _, repoAndTag := range repos {
			parsedRepo, parsedTag := parsers.ParseRepositoryTag(repoAndTag)
			if repoName == "" || repoName == parsedRepo {
				repoName = parsedRepo
				if parsedTag != "" {
					tags = append(tags, parsedTag)
				}
			} else if repoName != parsedRepo && !force {
				// the id belongs to multiple repos, like base:latest and user:test,
				// in that case return conflict
				return fmt.Errorf("Conflict, cannot delete image %s because it is tagged in multiple repositories, use -f to force", name)
			}
		}
	} else {
		tags = append(tags, tag)
	}

	if !first && len(tags) > 0 {
		return nil
	}

	if len(repos) <= 1 {
		if err := daemon.canDeleteImage(img.ID, force); err != nil {
			return err
		}
	}

	// Untag the current image
	for _, tag := range tags {
		tagDeleted, err := daemon.Repositories().Delete(repoName, tag)
		if err != nil {
			return err
		}
		if tagDeleted {
			out := &engine.Env{}
			out.Set("Untagged", repoName+":"+tag)
			imgs.Add(out)
			eng.Job("log", "untag", img.ID, "").Run()
		}
	}
	tags = daemon.Repositories().ByID()[img.ID]
	if (len(tags) <= 1 && repoName == "") || len(tags) == 0 {
		if len(byParents[img.ID]) == 0 {
			if err := daemon.Repositories().DeleteAll(img.ID); err != nil {
				return err
			}
			if err := daemon.Graph().Delete(img.ID); err != nil {
				return err
			}
			out := &engine.Env{}
			out.Set("Deleted", img.ID)
			imgs.Add(out)
			eng.Job("log", "delete", img.ID, "").Run()
			if img.Parent != "" && !noprune {
				err := daemon.DeleteImage(eng, img.Parent, imgs, false, force, noprune)
				if first {
					return err
				}

			}

		}
	}
	return nil
}

func (daemon *Daemon) canDeleteImage(imgID string, force bool) error {
	for _, container := range daemon.List() {
		parent, err := daemon.Repositories().LookupImage(container.Image)
		if err != nil {
			if daemon.Graph().IsNotExist(err) {
				return nil
			}
			return err
		}

		if err := parent.WalkHistory(func(p *image.Image) error {
			if imgID == p.ID {
				if container.IsRunning() {
					if force {
						return fmt.Errorf("Conflict, cannot force delete %s because the running container %s is using it, stop it and retry", utils.TruncateID(imgID), utils.TruncateID(container.ID))
					}
					return fmt.Errorf("Conflict, cannot delete %s because the running container %s is using it, stop it and use -f to force", utils.TruncateID(imgID), utils.TruncateID(container.ID))
				} else if !force {
					return fmt.Errorf("Conflict, cannot delete %s because the container %s is using it, use -f to force", utils.TruncateID(imgID), utils.TruncateID(container.ID))
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
