package dockerfile

import (
	"fmt"
	"strings"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/engine-api/types"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestEmptyDockerfile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-dockerfile-test")
	defer cleanup()

	createTestTempFile(c, contextDir, builder.DefaultDockerfileName, "", 0777)

	readAndCheckDockerfile(c, "emptyDockefile", contextDir, "", "The Dockerfile (Dockerfile) cannot be empty")
}

func (s *DockerSuite) TestSymlinkDockerfile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-dockerfile-test")
	defer cleanup()

	createTestSymlink(c, contextDir, builder.DefaultDockerfileName, "/etc/passwd")

	// The reason the error is "Cannot locate specified Dockerfile" is because
	// in the builder, the symlink is resolved within the context, therefore
	// Dockerfile -> /etc/passwd becomes etc/passwd from the context which is
	// a nonexistent file.
	expectedError := fmt.Sprintf("Cannot locate specified Dockerfile: %s", builder.DefaultDockerfileName)

	readAndCheckDockerfile(c, "symlinkDockerfile", contextDir, builder.DefaultDockerfileName, expectedError)
}

func (s *DockerSuite) TestDockerfileOutsideTheBuildContext(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-dockerfile-test")
	defer cleanup()

	expectedError := "Forbidden path outside the build context"

	readAndCheckDockerfile(c, "DockerfileOutsideTheBuildContext", contextDir, "../../Dockerfile", expectedError)
}

func (s *DockerSuite) TestNonExistingDockerfile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-dockerfile-test")
	defer cleanup()

	expectedError := "Cannot locate specified Dockerfile: Dockerfile"

	readAndCheckDockerfile(c, "NonExistingDockerfile", contextDir, "Dockerfile", expectedError)
}

func readAndCheckDockerfile(c *check.C, testName, contextDir, dockerfilePath, expectedError string) {
	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)

	if err != nil {
		c.Fatalf("Error when creating tar stream: %s", err)
	}

	defer func() {
		if err = tarStream.Close(); err != nil {
			c.Fatalf("Error when closing tar stream: %s", err)
		}
	}()

	context, err := builder.MakeTarSumContext(tarStream)

	if err != nil {
		c.Fatalf("Error when creating tar context: %s", err)
	}

	defer func() {
		if err = context.Close(); err != nil {
			c.Fatalf("Error when closing tar context: %s", err)
		}
	}()

	options := &types.ImageBuildOptions{
		Dockerfile: dockerfilePath,
	}

	b := &Builder{options: options, context: context}

	err = b.readDockerfile()

	if err == nil {
		c.Fatalf("No error when executing test: %s", testName)
	}

	if !strings.Contains(err.Error(), expectedError) {
		c.Fatalf("Wrong error message. Should be \"%s\". Got \"%s\"", expectedError, err.Error())
	}
}
