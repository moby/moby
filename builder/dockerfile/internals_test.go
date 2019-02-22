package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestEmptyDockerfile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-dockerfile-test")
	defer cleanup()

	createTestTempFile(t, contextDir, builder.DefaultDockerfileName, "", 0777)

	readAndCheckDockerfile(t, "emptyDockerfile", contextDir, "", "the Dockerfile (Dockerfile) cannot be empty")
}

func TestSymlinkDockerfile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-dockerfile-test")
	defer cleanup()

	createTestSymlink(t, contextDir, builder.DefaultDockerfileName, "/etc/passwd")

	// The reason the error is "Cannot locate specified Dockerfile" is because
	// in the builder, the symlink is resolved within the context, therefore
	// Dockerfile -> /etc/passwd becomes etc/passwd from the context which is
	// a nonexistent file.
	expectedError := fmt.Sprintf("Cannot locate specified Dockerfile: %s. This can happen if you have a .dockerignore file that is ignoring the path to the dockerfile.", builder.DefaultDockerfileName)

	readAndCheckDockerfile(t, "symlinkDockerfile", contextDir, builder.DefaultDockerfileName, expectedError)
}

func TestDockerfileOutsideTheBuildContext(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-dockerfile-test")
	defer cleanup()

	expectedError := "Forbidden path outside the build context: ../../Dockerfile ()"
	if runtime.GOOS == "windows" {
		expectedError = "failed to resolve scoped path ../../Dockerfile ()"
	}

	readAndCheckDockerfile(t, "DockerfileOutsideTheBuildContext", contextDir, "../../Dockerfile", expectedError)
}

func TestNonExistingDockerfile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-dockerfile-test")
	defer cleanup()

	expectedError := "Cannot locate specified Dockerfile: Dockerfile"

	readAndCheckDockerfile(t, "NonExistingDockerfile", contextDir, "Dockerfile", expectedError)
}

func readAndCheckDockerfile(t *testing.T, testName, contextDir, dockerfilePath, expectedError string) {
	if runtime.GOOS != "windows" {
		skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	}
	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)
	assert.NilError(t, err)

	defer func() {
		if err = tarStream.Close(); err != nil {
			t.Fatalf("Error when closing tar stream: %s", err)
		}
	}()

	if dockerfilePath == "" { // handled in BuildWithContext
		dockerfilePath = builder.DefaultDockerfileName
	}

	config := backend.BuildConfig{
		Options: &types.ImageBuildOptions{Dockerfile: dockerfilePath},
		Source:  tarStream,
	}
	_, _, err = remotecontext.Detect(config)
	assert.Check(t, is.ErrorContains(err, expectedError))
}

func TestCopyRunConfig(t *testing.T) {
	defaultEnv := []string{"foo=1"}
	defaultCmd := []string{"old"}

	var testcases = []struct {
		doc       string
		modifiers []runConfigModifier
		expected  *container.Config
	}{
		{
			doc:       "Set the command",
			modifiers: []runConfigModifier{withCmd([]string{"new"})},
			expected: &container.Config{
				Cmd: []string{"new"},
				Env: defaultEnv,
			},
		},
		{
			doc:       "Set the command to a comment",
			modifiers: []runConfigModifier{withCmdComment("comment", runtime.GOOS)},
			expected: &container.Config{
				Cmd: append(defaultShellForOS(runtime.GOOS), "#(nop) ", "comment"),
				Env: defaultEnv,
			},
		},
		{
			doc: "Set the command and env",
			modifiers: []runConfigModifier{
				withCmd([]string{"new"}),
				withEnv([]string{"one", "two"}),
			},
			expected: &container.Config{
				Cmd: []string{"new"},
				Env: []string{"one", "two"},
			},
		},
	}

	for _, testcase := range testcases {
		runConfig := &container.Config{
			Cmd: defaultCmd,
			Env: defaultEnv,
		}
		runConfigCopy := copyRunConfig(runConfig, testcase.modifiers...)
		assert.Check(t, is.DeepEqual(testcase.expected, runConfigCopy), testcase.doc)
		// Assert the original was not modified
		assert.Check(t, runConfig != runConfigCopy, testcase.doc)
	}

}

func fullMutableRunConfig() *container.Config {
	return &container.Config{
		Cmd: []string{"command", "arg1"},
		Env: []string{"env1=foo", "env2=bar"},
		ExposedPorts: nat.PortSet{
			"1000/tcp": {},
			"1001/tcp": {},
		},
		Volumes: map[string]struct{}{
			"one": {},
			"two": {},
		},
		Entrypoint: []string{"entry", "arg1"},
		OnBuild:    []string{"first", "next"},
		Labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Shell: []string{"shell", "-c"},
	}
}

func TestDeepCopyRunConfig(t *testing.T) {
	runConfig := fullMutableRunConfig()
	copy := copyRunConfig(runConfig)
	assert.Check(t, is.DeepEqual(fullMutableRunConfig(), copy))

	copy.Cmd[1] = "arg2"
	copy.Env[1] = "env2=new"
	copy.ExposedPorts["10002"] = struct{}{}
	copy.Volumes["three"] = struct{}{}
	copy.Entrypoint[1] = "arg2"
	copy.OnBuild[0] = "start"
	copy.Labels["label3"] = "value3"
	copy.Shell[0] = "sh"
	assert.Check(t, is.DeepEqual(fullMutableRunConfig(), runConfig))
}
