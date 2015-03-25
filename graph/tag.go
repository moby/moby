package graph

import (
	"fmt"

	"github.com/docker/docker/engine"
)

func (s *TagStore) CmdTag(job *engine.Job) error {
	if len(job.Args) != 2 && len(job.Args) != 3 {
		return fmt.Errorf("Usage: %s IMAGE REPOSITORY [TAG]\n", job.Name)
	}
	var tag string
	if len(job.Args) == 3 {
		tag = job.Args[2]
	}
	return s.Set(job.Args[1], tag, job.Args[0], job.GetenvBool("force"))
}
