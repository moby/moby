package graph

import (
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/parsers"
)

// CmdTag assigns a new name and tag to an existing image. If the tag already exists,
// it is changed and the image previously referenced by the tag loses that reference.
// This may cause the old image to be garbage-collected if its reference count reaches zero.
//
// Syntax: image_tag NEWNAME OLDNAME
// Example: image_tag shykes/myapp:latest shykes/myapp:1.42.0
func (s *TagStore) CmdTag(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("usage: %s NEWNAME OLDNAME", job.Name)
	}
	var (
		newName = job.Args[0]
		oldName = job.Args[1]
	)
	newRepo, newTag := parsers.ParseRepositoryTag(newName)
	// FIXME: Set should either parse both old and new name, or neither.
	// 	the current prototype is inconsistent.
	if err := s.Set(newRepo, newTag, oldName, true); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

// FIXME: merge into CmdTag above, and merge "image_tag" and "tag" into a single job.
func (s *TagStore) CmdTagLegacy(job *engine.Job) engine.Status {
	if len(job.Args) != 2 && len(job.Args) != 3 {
		return job.Errorf("Usage: %s IMAGE REPOSITORY [TAG]\n", job.Name)
	}
	var tag string
	if len(job.Args) == 3 {
		tag = job.Args[2]
	}
	if err := s.Set(job.Args[1], tag, job.Args[0], job.GetenvBool("force")); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}
