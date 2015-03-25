package graph

import (
	"fmt"
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/utils"
)

func (s *TagStore) CmdHistory(job *engine.Job) error {
	if n := len(job.Args); n != 1 {
		return fmt.Errorf("Usage: %s IMAGE", job.Name)
	}
	name := job.Args[0]
	foundImage, err := s.LookupImage(name)
	if err != nil {
		return err
	}

	lookupMap := make(map[string][]string)
	for name, repository := range s.Repositories {
		for tag, id := range repository {
			// If the ID already has a reverse lookup, do not update it unless for "latest"
			if _, exists := lookupMap[id]; !exists {
				lookupMap[id] = []string{}
			}
			lookupMap[id] = append(lookupMap[id], utils.ImageReference(name, tag))
		}
	}

	outs := engine.NewTable("Created", 0)
	err = foundImage.WalkHistory(func(img *image.Image) error {
		out := &engine.Env{}
		out.SetJson("Id", img.ID)
		out.SetInt64("Created", img.Created.Unix())
		out.Set("CreatedBy", strings.Join(img.ContainerConfig.Cmd, " "))
		out.SetList("Tags", lookupMap[img.ID])
		out.SetInt64("Size", img.Size)
		outs.Add(out)
		return nil
	})
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return err
	}
	return nil
}
