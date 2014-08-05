package graph

import (
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
)

func (s *TagStore) CmdHistory(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s IMAGE", job.Name)
	}
	name := job.Args[0]
	foundImage, err := s.LookupImage(name)
	if err != nil {
		return job.Error(err)
	}

	lookupMap := make(map[string][]string)
	for name, repository := range s.Repositories {
		for tag, id := range repository {
			// If the ID already has a reverse lookup, do not update it unless for "latest"
			if _, exists := lookupMap[id]; !exists {
				lookupMap[id] = []string{}
			}
			lookupMap[id] = append(lookupMap[id], name+":"+tag)
		}
	}

	outs := engine.NewTable("Created", 0)
	err = foundImage.WalkHistory(func(img *image.Image) error {
		out := &engine.Env{}
		out.Set("Id", img.ID)
		out.SetInt64("Created", img.Created.Unix())
		out.Set("CreatedBy", strings.Join(img.ContainerConfig.Cmd, " "))
		out.SetList("Tags", lookupMap[img.ID])
		out.SetInt64("Size", img.Size)
		outs.Add(out)
		return nil
	})
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}
