package build

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/pkg/fileutils"
)

// ReadDockerignore reads the .dockerignore file in the context directory and
// returns the list of paths to exclude
func ReadDockerignore(contextDir string) ([]string, error) {
	var excludes []string

	f, err := os.Open(filepath.Join(contextDir, ".dockerignore"))
	switch {
	case os.IsNotExist(err):
		return excludes, nil
	case err != nil:
		return nil, err
	}
	defer f.Close()

	return dockerignore.ReadAll(f)
}

// TrimBuildFilesFromExcludes removes the named Dockerfile and .dockerignore from
// the list of excluded files. The daemon will remove them from the final context
// but they must be in available in the context when passed to the API.
func TrimBuildFilesFromExcludes(excludes []string, dockerfile string, dockerfileFromStdin bool) []string {
	if keep, _ := fileutils.Matches(".dockerignore", excludes); keep {
		excludes = append(excludes, "!.dockerignore")
	}
	if keep, _ := fileutils.Matches(dockerfile, excludes); keep && !dockerfileFromStdin {
		excludes = append(excludes, "!"+dockerfile)
	}
	return excludes
}
