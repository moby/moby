package graph

import (
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
)

func (s *TagStore) CmdViz(job *engine.Job) engine.Status {
	images, _ := s.graph.Map()
	if images == nil {
		return engine.StatusOK
	}
	job.Stdout.Write([]byte("digraph docker {\n"))

	var (
		parentImage *image.Image
		err         error
	)
	for _, image := range images {
		parentImage, err = image.GetParent()
		if err != nil {
			return job.Errorf("Error while getting parent image: %v", err)
		}
		if parentImage != nil {
			job.Stdout.Write([]byte(" \"" + parentImage.ID + "\" -> \"" + image.ID + "\"\n"))
		} else {
			job.Stdout.Write([]byte(" base -> \"" + image.ID + "\" [style=invis]\n"))
		}
	}

	for id, repos := range s.GetRepoRefs() {
		job.Stdout.Write([]byte(" \"" + id + "\" [label=\"" + id + "\\n" + strings.Join(repos, "\\n") + "\",shape=box,fillcolor=\"paleturquoise\",style=\"filled,rounded\"];\n"))
	}
	job.Stdout.Write([]byte(" base [style=invisible]\n}\n"))
	return engine.StatusOK
}
