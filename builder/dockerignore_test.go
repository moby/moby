package builder

import (
	"io/ioutil"
	"log"
	"os"
	"sort"

	"github.com/go-check/check"
)

const shouldStayFilename = "should_stay"

func extractFilenames(files []os.FileInfo) []string {
	filenames := make([]string, len(files), len(files))

	for i, file := range files {
		filenames[i] = file.Name()
	}

	return filenames
}

func checkDirectory(c *check.C, dir string, expectedFiles []string) {
	files, err := ioutil.ReadDir(dir)

	if err != nil {
		c.Fatalf("Could not read directory: %s", err)
	}

	if len(files) != len(expectedFiles) {
		log.Fatalf("Directory should contain exactly %d file(s), got %d", len(expectedFiles), len(files))
	}

	filenames := extractFilenames(files)
	sort.Strings(filenames)
	sort.Strings(expectedFiles)

	for i, filename := range filenames {
		if filename != expectedFiles[i] {
			c.Fatalf("File %s should be in the directory, got: %s", expectedFiles[i], filename)
		}
	}
}

func executeProcess(c *check.C, contextDir string) {
	modifiableCtx := &tarSumContext{root: contextDir}
	ctx := DockerIgnoreContext{ModifiableContext: modifiableCtx}

	err := ctx.Process([]string{DefaultDockerfileName})

	if err != nil {
		c.Fatalf("Error when executing Process: %s", err)
	}
}

func (s *DockerSuite) TestProcessShouldRemoveDockerfileDockerignore(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-dockerignore-process-test")
	defer cleanup()

	createTestTempFile(c, contextDir, shouldStayFilename, testfileContents, 0777)
	createTestTempFile(c, contextDir, dockerignoreFilename, "Dockerfile\n.dockerignore", 0777)
	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	executeProcess(c, contextDir)

	checkDirectory(c, contextDir, []string{shouldStayFilename})

}

func (s *DockerSuite) TestProcessNoDockerignore(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-dockerignore-process-test")
	defer cleanup()

	createTestTempFile(c, contextDir, shouldStayFilename, testfileContents, 0777)
	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	executeProcess(c, contextDir)

	checkDirectory(c, contextDir, []string{shouldStayFilename, DefaultDockerfileName})

}

func (s *DockerSuite) TestProcessShouldLeaveAllFiles(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-dockerignore-process-test")
	defer cleanup()

	createTestTempFile(c, contextDir, shouldStayFilename, testfileContents, 0777)
	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)
	createTestTempFile(c, contextDir, dockerignoreFilename, "input1\ninput2", 0777)

	executeProcess(c, contextDir)

	checkDirectory(c, contextDir, []string{shouldStayFilename, DefaultDockerfileName, dockerignoreFilename})

}
