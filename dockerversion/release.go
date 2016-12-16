package dockerversion

import (
	"encoding/json"
	"os"

	"github.com/docker/docker/api/types"
)

// DockerReleaseFile represents the content of the docker-release file
type DockerReleaseFile struct {
	FormatVersion string
	DockerVersion types.DockerVersion
}

// DockerRelease returns the DockerReleaseFile
func DockerRelease() *DockerReleaseFile {
	if dockerReleaseFile, err := os.Open(dockerReleasePath); err == nil {
		var dockerReleaseFileContent DockerReleaseFile
		if err := json.NewDecoder(dockerReleaseFile).Decode(&dockerReleaseFileContent); err == nil {
			return &dockerReleaseFileContent
		}
	}
	return nil
}
