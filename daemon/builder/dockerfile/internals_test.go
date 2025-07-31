package dockerfile

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/moby/go-archive"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/builder"
	"github.com/moby/moby/v2/daemon/builder/remotecontext"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestEmptyDockerfile(t *testing.T) {
	contextDir := t.TempDir()
	createTestTempFile(t, contextDir, builder.DefaultDockerfileName, "", 0o777)
	readAndCheckDockerfile(t, "emptyDockerfile", contextDir, "", "the Dockerfile (Dockerfile) cannot be empty")
}

func TestSymlinkDockerfile(t *testing.T) {
	contextDir := t.TempDir()

	createTestSymlink(t, contextDir, builder.DefaultDockerfileName, "/etc/passwd")

	// The reason the error is "Cannot locate specified Dockerfile" is because
	// in the builder, the symlink is resolved within the context, therefore
	// Dockerfile -> /etc/passwd becomes etc/passwd from the context which is
	// a nonexistent file.
	expectedError := fmt.Sprintf("Cannot locate specified Dockerfile: %s", builder.DefaultDockerfileName)

	readAndCheckDockerfile(t, "symlinkDockerfile", contextDir, builder.DefaultDockerfileName, expectedError)
}

func TestDockerfileOutsideTheBuildContext(t *testing.T) {
	contextDir := t.TempDir()

	expectedError := "path outside the build context: ../../Dockerfile ()"
	if runtime.GOOS == "windows" {
		expectedError = "failed to resolve scoped path ../../Dockerfile ()"
	}

	readAndCheckDockerfile(t, "DockerfileOutsideTheBuildContext", contextDir, "../../Dockerfile", expectedError)
}

func TestNonExistingDockerfile(t *testing.T) {
	contextDir := t.TempDir()
	const expectedError = "Cannot locate specified Dockerfile: Dockerfile"
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
		Options: &build.ImageBuildOptions{Dockerfile: dockerfilePath},
		Source:  tarStream,
	}
	_, _, err = remotecontext.Detect(config)
	assert.Check(t, is.ErrorContains(err, expectedError))
}

func TestCopyRunConfig(t *testing.T) {
	defaultEnv := []string{"foo=1"}
	defaultCmd := []string{"old"}

	testcases := []struct {
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
		ExposedPorts: container.PortSet{
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
	ctrCfg := copyRunConfig(runConfig)
	assert.Check(t, is.DeepEqual(fullMutableRunConfig(), ctrCfg))

	ctrCfg.Cmd[1] = "arg2"
	ctrCfg.Env[1] = "env2=new"
	ctrCfg.ExposedPorts["10002"] = struct{}{}
	ctrCfg.Volumes["three"] = struct{}{}
	ctrCfg.Entrypoint[1] = "arg2"
	ctrCfg.OnBuild[0] = "start"
	ctrCfg.Labels["label3"] = "value3"
	ctrCfg.Shell[0] = "sh"
	assert.Check(t, is.DeepEqual(fullMutableRunConfig(), runConfig))
}

type MockRWLayer struct{}

func (l *MockRWLayer) Release() error { return nil }
func (l *MockRWLayer) Root() string   { return "" }
func (l *MockRWLayer) Commit() (builder.ROLayer, error) {
	return &MockROLayer{
		diffID: "sha256:1234",
	}, nil
}

type MockROLayer struct {
	diffID layer.DiffID
}

func (l *MockROLayer) ContentStoreDigest() digest.Digest    { return "" }
func (l *MockROLayer) Release() error                       { return nil }
func (l *MockROLayer) NewRWLayer() (builder.RWLayer, error) { return nil, nil }
func (l *MockROLayer) DiffID() layer.DiffID                 { return l.diffID }

func getMockBuildBackend() builder.Backend {
	return &MockBackend{}
}

func TestExportImage(t *testing.T) {
	ds := newDispatchState(NewBuildArgs(map[string]*string{}))
	parentImage := &image.Image{
		V1Image: image.V1Image{
			OS:           "linux",
			Architecture: "arm64",
			Variant:      "v8",
		},
	}
	b := &Builder{
		imageSources: getMockImageSource(nil, nil, nil),
		docker:       getMockBuildBackend(),
	}
	err := b.exportImage(context.TODO(), ds, &MockRWLayer{}, parentImage, &container.Config{})
	assert.NilError(t, err)
}
