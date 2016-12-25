package builder

import (
	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/pkg/fileutils"
)

// DockerIgnoreContext wraps a ModifiableContext to add a method
// for handling the .dockerignore file at the root of the context.
type DockerIgnoreContext struct {
	ModifiableContext
}

// Process reads the .dockerignore file at the root of the embedded context.
// If .dockerignore does not exist in the context, then nil is returned.
//
// It can take a list of files to be removed after .dockerignore is removed.
// This is used for server-side implementations of builders that need to send
// the .dockerignore file as well as the special files specified in filesToRemove,
// but expect them to be excluded from the context after they were processed.
//
// For example, server-side Dockerfile builders are expected to pass in the name
// of the Dockerfile to be removed after it was parsed.
//
// TODO: Don't require a ModifiableContext (use Context instead) and don't remove
// files, instead handle a list of files to be excluded from the context.
func (c DockerIgnoreContext) Process(ignoreFile string, filesToRemove []string) error {
	f, err := c.Open(ignoreFile)
	if err != nil {
		return err
	}
	excludes, _ := dockerignore.ReadAll(f)
	f.Close()
	filesToRemove = append([]string{ignoreFile}, filesToRemove...)
	for _, fileToRemove := range filesToRemove {
		rm, _ := fileutils.Matches(fileToRemove, excludes)
		if rm {
			c.Remove(fileToRemove)
		}
	}
	return nil
}
